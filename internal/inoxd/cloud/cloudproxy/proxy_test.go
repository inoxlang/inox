package cloudproxy

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/permkind"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/inoxlang/inox/internal/globals/ws_ns"
	"github.com/inoxlang/inox/internal/inoxconsts"
	"github.com/inoxlang/inox/internal/inoxd/cloud/cloudproxy/inoxdconn"
	"github.com/inoxlang/inox/internal/inoxd/consts"
	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/assert"
)

func TestInoxdConnection(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	//TODO: fix implementation

	inoxdconn.RegisterTypesInGob()

	fls := fs_ns.NewMemFilesystem(1_000_000)
	goCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	port := 6000

	var earlyErr atomic.Value

	go func() {
		err := Run(CloudProxyArgs{
			Config: CloudProxyConfig{
				CloudDataDir:                 "/",
				AnonymousAccountDatabasePath: "/anon-db.kv",
				Port:                         port,
			},
			Filesystem: fls,
			OutW:       os.Stdout,
			ErrW:       os.Stdout,
			GoContext:  goCtx,
		})

		if err != nil {
			earlyErr.Store(err)
		}
	}()

	//wait for the cloud proxy to start
	time.Sleep(time.Second)

	if err := earlyErr.Load(); err != nil {
		assert.FailNow(t, err.(error).Error())
	}

	dialer := *websocket.DefaultDialer
	dialer.TLSClientConfig = &tls.Config{
		InsecureSkipVerify: true,
	}

	socket, _, err := dialer.Dial("wss://localhost:"+strconv.Itoa(port)+consts.PROXY__INOXD_WEBSOCKET_ENDPOINT, nil)
	if !assert.NoError(t, err) {
		return
	}
	defer socket.Close()

	//send hello message.
	helloMsg := inoxdconn.Message{
		ULID:  ulid.Make(),
		Inner: inoxdconn.Hello{},
	}

	err = socket.WriteMessage(websocket.BinaryMessage, inoxdconn.MustEncodeMessage(helloMsg))
	if !assert.NoError(t, err) {
		return
	}

	//wait for the cloud proxy to answer.
	time.Sleep(100 * time.Millisecond)

	msgType, payload, err := socket.ReadMessage()
	if !assert.NoError(t, err) {
		return
	}
	if !assert.Equal(t, websocket.BinaryMessage, msgType) {
		return
	}

	var ackMsg inoxdconn.Message
	if !assert.NoError(t, inoxdconn.DecodeMessage(payload, &ackMsg)) {
		return
	}

	if !assert.IsType(t, ackMsg.Inner, inoxdconn.Ack{}) {
		return
	}
	ack := ackMsg.Inner.(inoxdconn.Ack)
	assert.Equal(t, helloMsg.ULID, ack.AcknowledgedMessage)
}

func TestAccountCreation(t *testing.T) {
	t.Skip("manual test")
	username := "<Github username>"

	//outW := io.Discard
	//errW := io.Discard
	outW := os.Stdout
	errW := os.Stdout

	config := CloudProxyConfig{
		CloudDataDir:                 "/",
		AnonymousAccountDatabasePath: "/db.kv",
		Port:                         inoxconsts.DEFAULT_PROJECT_SERVER_PORT_INT,
	}
	goctx, cancel := context.WithTimeout(context.Background(), 29*time.Second)
	defer cancel()

	var returnedError atomic.Value

	fls := fs_ns.NewMemFilesystem(1_000_000)

	go func() {
		args := CloudProxyArgs{
			Config:     config,
			OutW:       outW,
			ErrW:       errW,
			GoContext:  goctx,
			Filesystem: fls,
		}
		returnedError.Store(Run(args))
	}()

	time.Sleep(time.Second) //wait for the cloud proxy to start.

	host := core.Host("wss://localhost:" + inoxconsts.DEFAULT_PROJECT_SERVER_PORT)

	ctx := core.NewContextWithEmptyState(core.ContextConfig{
		Permissions: []core.Permission{
			core.WebsocketPermission{Kind_: permkind.Read, Endpoint: host},
			core.WebsocketPermission{Kind_: permkind.Write, Endpoint: host},
		},
		Limits:              []core.Limit{{Name: ws_ns.WS_SIMUL_CONN_TOTAL_LIMIT_NAME, Kind: core.TotalLimit, Value: 5}},
		ParentStdLibContext: goctx,
	}, nil)

	insecure := true
	registrationEndpointWithQuery := host.URLWithPath(ACCOUNT_REGISTRATION_URL_PATH) + "?" + ACCOUNT_REGISTRATION_HOSTER_PARAM_NAME + "=Github"

	var accountToken string

	//register account
	{
		socket, err := ws_ns.WebsocketConnect(ws_ns.WebsocketConnectParams{
			Ctx:      ctx,
			URL:      registrationEndpointWithQuery,
			Insecure: insecure,
		})

		if !assert.NoError(t, err) {
			return
		}
		defer socket.Close()

		_, p, err := socket.ReadMessage(ctx)
		if !assert.NoError(t, err) {
			return
		}
		text := string(p)
		assert.True(t, strings.HasPrefix(text, "explanation:"))

		fmt.Fprintln(outW, text)

		//give human tester enough time to complete the challenge
		time.Sleep(15 * time.Second)
		if !assert.NoError(t, socket.WriteMessage(ctx, ws_ns.WebsocketTextMessage, []byte(username))) {
			return
		}

		_, p, err = socket.ReadMessage(ctx)
		if !assert.NoError(t, err) {
			return
		}
		text = string(p)
		assert.True(t, strings.HasPrefix(text, "token:"))
		fmt.Fprintln(outW, text)
		accountToken = strings.TrimPrefix(text, "token:")

		//acknowledge token reception
		assert.NoError(t, socket.WriteMessage(ctx, ws_ns.WebsocketTextMessage, []byte("ack:token")))

		//wait for the database to persist the account.
		time.Sleep(time.Second)
	}

	{
		//connection without account token should not work
		socket, err := ws_ns.WebsocketConnect(ws_ns.WebsocketConnectParams{
			Ctx:      ctx,
			URL:      host.URLWithPath("/"),
			Insecure: insecure,
		})

		if !assert.Error(t, err) {
			socket.Close()
			return
		}
	}

	{
		//connection with the account token should work
		socket, err := ws_ns.WebsocketConnect(ws_ns.WebsocketConnectParams{
			Ctx:      ctx,
			URL:      host.URLWithPath("/"),
			Insecure: insecure,
			RequestHeader: http.Header{
				ACCOUNT_TOKEN_HEADER_NAME: []string{accountToken},
			},
		})

		if !assert.NoError(t, err) {
			return
		}

		defer socket.Close()

		time.Sleep(time.Second)

		//still connected.
		assert.False(t, socket.IsClosedOrClosing())
	}
}
