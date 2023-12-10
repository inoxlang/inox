package projectserver

import (
	"encoding/json"
	"fmt"
	"io"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/inoxlang/inox/internal/core"
	fs_ns "github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/inoxlang/inox/internal/globals/http_ns"
	"github.com/inoxlang/inox/internal/globals/net_ns"
	"github.com/inoxlang/inox/internal/globals/s3_ns"
	"github.com/inoxlang/inox/internal/globals/ws_ns"
	"github.com/inoxlang/inox/internal/memds"
	"github.com/inoxlang/inox/internal/permkind"
	"github.com/inoxlang/inox/internal/projectserver/jsonrpc"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

//utils

func createTestServerAndClient(t *testing.T) (*core.Context, *testClient, bool) {

	if !core.AreDefaultScriptLimitsSet() {
		core.SetDefaultScriptLimits([]core.Limit{
			{Name: fs_ns.FS_READ_LIMIT_NAME, Kind: core.ByteRateLimit, Value: 100_000_000},
			{Name: fs_ns.FS_WRITE_LIMIT_NAME, Kind: core.ByteRateLimit, Value: 100_000_000},

			{Name: fs_ns.FS_NEW_FILE_RATE_LIMIT_NAME, Kind: core.SimpleRateLimit, Value: 100},
			{Name: fs_ns.FS_TOTAL_NEW_FILE_LIMIT_NAME, Kind: core.TotalLimit, Value: 10_000},

			{Name: http_ns.HTTP_REQUEST_RATE_LIMIT_NAME, Kind: core.SimpleRateLimit, Value: 100},
			{Name: ws_ns.WS_SIMUL_CONN_TOTAL_LIMIT_NAME, Kind: core.TotalLimit, Value: 10},
			{Name: net_ns.TCP_SIMUL_CONN_TOTAL_LIMIT_NAME, Kind: core.TotalLimit, Value: 10},

			{Name: s3_ns.OBJECT_STORAGE_REQUEST_RATE_LIMIT_NAME, Kind: core.SimpleRateLimit, Value: 50},

			{Name: core.THREADS_SIMULTANEOUS_INSTANCES_LIMIT_NAME, Kind: core.TotalLimit, Value: 5},
		})
	}

	projectsDirFilesystem := fs_ns.NewMemFilesystem(10_000_000)
	projectsDir := core.Path("/")

	//create context & state
	perms := []core.Permission{
		//TODO: change path pattern
		core.FilesystemPermission{Kind_: permkind.Read, Entity: core.PathPattern("/...")},
		core.FilesystemPermission{Kind_: permkind.Write, Entity: core.PathPattern("/...")},
		core.FilesystemPermission{Kind_: permkind.Delete, Entity: core.PathPattern("/...")},

		core.WebsocketPermission{Kind_: permkind.Provide},
		core.HttpPermission{Kind_: permkind.Provide, Entity: core.ANY_HTTPS_HOST_PATTERN},
		core.HttpPermission{Kind_: permkind.Provide, Entity: core.HostPattern("https://**:8080")},

		core.HttpPermission{Kind_: permkind.Read, AnyEntity: true},
		core.HttpPermission{Kind_: permkind.Write, AnyEntity: true},
		core.HttpPermission{Kind_: permkind.Delete, AnyEntity: true},

		core.LThreadPermission{Kind_: permkind.Create},
	}

	perms = append(perms, core.GetDefaultGlobalVarPermissions()...)

	ctx := core.NewContext(core.ContextConfig{
		Permissions: perms,
		Filesystem:  projectsDirFilesystem,
	})

	state := core.NewGlobalState(ctx)
	state.Out = io.Discard
	state.Logger = zerolog.Nop()
	state.OutputFieldsInitialized.Store(true)

	//configure server

	client := newTestClient()

	conf := LSPServerConfiguration{
		UseContextLogger:      true,
		ProjectMode:           true,
		ProjectsDir:           projectsDir,
		ProjectsDirFilesystem: projectsDirFilesystem,
		MessageReaderWriter:   client.msgReaderWriter,
		OnSession: func(rpcCtx *core.Context, session *jsonrpc.Session) error {
			sessionCtx := core.NewContext(core.ContextConfig{
				Permissions:          rpcCtx.GetGrantedPermissions(),
				ForbiddenPermissions: rpcCtx.GetForbiddenPermissions(),
				Limits:               core.GetDefaultScriptLimits(),

				ParentContext: rpcCtx,
			})
			tempState := core.NewGlobalState(sessionCtx)
			tempState.Out = io.Discard
			tempState.Logger = zerolog.Nop()
			tempState.OutputFieldsInitialized.Store(true)
			session.SetContextOnce(sessionCtx)
			return nil
		},
	}

	var hasErr atomic.Bool

	go func() {
		err := StartLSPServer(ctx, conf)
		if !assert.NoError(t, err) {
			hasErr.Store(true)
		}
	}()

	time.Sleep(10 * time.Millisecond)
	if hasErr.Load() {
		return nil, nil, false
	}

	return ctx, client, true
}

type testClient struct {
	closed           atomic.Bool
	outgoingMessages chan []byte
	incomingMessages *memds.TSArrayQueue[[]byte]
	msgReaderWriter  jsonrpc.MessageReaderWriter
}

func newTestClient() *testClient {

	client := &testClient{
		outgoingMessages: make(chan []byte, 20),
		incomingMessages: memds.NewTSArrayQueue[[]byte](),
	}
	client.msgReaderWriter = &jsonrpc.FnMessageReaderWriter{
		ReadMessageFn: func() (msg []byte, err error) {
			if client.closed.Load() {
				return nil, io.EOF
			}

			return <-client.outgoingMessages, nil
		},
		WriteMessageFn: func(msg []byte) error {
			if client.closed.Load() {
				return io.EOF
			}

			client.incomingMessages.Enqueue(msg)
			return nil
		},
		CloseFn: func() error {
			client.closed.Store(true)
			return nil
		},
	}

	return client
}

func (s *testClient) close() {
	s.closed.Store(true)
}

// sendRequest sends a request to the server, RequestMessage.ID & RequestMessage.BaseMessage
// are set by the callee. After the request is sent sendRequest waits for 50ms.
func (s *testClient) sendRequest(req jsonrpc.RequestMessage) {
	req.BaseMessage = jsonrpc.BaseMessage{Jsonrpc: jsonrpc.JSONRPC_VERSION}
	req.ID = uuid.New()

	reqBytes, err := json.Marshal(req)
	if err != nil {
		panic(err)
	}

	s.outgoingMessages <- reqBytes
	time.Sleep(50 * time.Millisecond)
}

// sendNotif sends a notification to the server, NotificationMessage.BaseMessage
// is set by the callee. After the notification is sent sendNotif waits for 50ms.
func (s *testClient) sendNotif(notif jsonrpc.NotificationMessage) {
	notif.BaseMessage = jsonrpc.BaseMessage{Jsonrpc: jsonrpc.JSONRPC_VERSION}

	notifBytes, err := json.Marshal(notif)
	if err != nil {
		panic(err)
	}

	s.outgoingMessages <- notifBytes
	time.Sleep(50 * time.Millisecond)
}

func (s *testClient) dequeueLastMessage() (any, bool) {
	bytes, ok := s.incomingMessages.Peek()
	if !ok {
		return nil, false
	}

	var responseMessage jsonrpc.ResponseMessage

	if err := json.Unmarshal(bytes, &responseMessage); err == nil {
		return responseMessage, true
	}

	var notifMessage jsonrpc.NotificationMessage

	if err := json.Unmarshal(bytes, &notifMessage); err == nil {
		return notifMessage, true
	}

	var requestMessage jsonrpc.RequestMessage

	if err := json.Unmarshal(bytes, &requestMessage); err == nil {
		return requestMessage, true
	}

	panic(fmt.Errorf("invalid message: %s", string(bytes)))
}
