package inoxprocess

import (
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/url"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/inoxlang/inox/internal/globals/http_ns"
	"github.com/inoxlang/inox/internal/globals/net_ns"
	nettypes "github.com/inoxlang/inox/internal/net_types"
	"github.com/inoxlang/inox/internal/permkind"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/stretchr/testify/assert"
)

func TestControlClient(t *testing.T) {
	controlServerURL := utils.Must(url.Parse("wss://localhost:8310"))
	permissiveSocketCountLimit := core.MustMakeNotDecrementingLimit(net_ns.WS_SIMUL_CONN_TOTAL_LIMIT_NAME, 10_000)

	ctx := core.NewContexWithEmptyState(core.ContextConfig{
		Permissions: []core.Permission{
			core.WebsocketPermission{Kind_: permkind.Provide},
			core.WebsocketPermission{Kind_: permkind.Read, Endpoint: core.Host("wss://localhost:8310")},
			core.WebsocketPermission{Kind_: permkind.Write, Endpoint: core.Host("wss://localhost:8310")},
		},
		Filesystem: fs_ns.NewMemFilesystem(10_000),
		Limits:     []core.Limit{permissiveSocketCountLimit},
	}, os.Stdout)
	defer ctx.CancelGracefully()

	server, err := net_ns.NewWebsocketServer(ctx)

	if !assert.NoError(t, err) {
		return
	}

	cert, key, err := http_ns.GenerateSelfSignedCertAndKey()

	if !assert.NoError(t, err) {
		return
	}

	var receivedMessagePayloads [][]byte
	var receivedMessageTypes []net_ns.WebsocketMessageType
	var errors []error
	var lock sync.Mutex

	token := MakeControlledProcessToken()

	httpServer, err := http_ns.NewGolangHttpServer(ctx, http_ns.GolangHttpServerConfig{
		Addr:           "localhost:8310",
		PemEncodedCert: string(pem.EncodeToMemory(cert)),
		PemEncodedKey:  string(pem.EncodeToMemory(key)),
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			conn, err := server.UpgradeGoValues(w, r, func(remoteAddrPort nettypes.RemoteAddrWithPort, remoteAddr nettypes.RemoteIpAddr, currentConns []*net_ns.WebsocketConnection) error {
				return nil
			})

			if err != nil {
				lock.Lock()
				errors = append(errors, err)
				lock.Unlock()

				return
			}

			conn.SetPingHandler(ctx, func(data string) error {
				lock.Lock()
				t.Log("receive ping")
				receivedMessageTypes = append(receivedMessageTypes, net_ns.WebsocketPingMessage)
				receivedMessagePayloads = append(receivedMessagePayloads, []byte(data))
				lock.Unlock()
				return nil
			})

			for !conn.IsClosedOrClosing() {
				msgtype, p, err := conn.ReadMessage(ctx)
				if err != nil {
					conn.Close()
					lock.Lock()
					errors = append(errors, err)
					lock.Unlock()
					return
				}
				lock.Lock()
				receivedMessageTypes = append(receivedMessageTypes, msgtype)
				receivedMessagePayloads = append(receivedMessagePayloads, p)
				lock.Unlock()
			}
		}),
	})
	defer httpServer.Close()

	if !assert.NoError(t, err) {
		return
	}

	start := time.Now()
	go httpServer.ListenAndServeTLS("", "")

	client, err := ConnectToProcessControlServer(ctx, controlServerURL, token)
	if !assert.NoError(t, err) {
		return
	}
	defer client.Conn().Close()

	go client.StartControl()

	time.Sleep(time.Second)

	lock.Lock()
	defer lock.Unlock()

	t.Log(errors)

	if !assert.NotEmpty(t, receivedMessageTypes) {
		return
	}

	firstMsgType := receivedMessageTypes[0]
	if !assert.Equal(t, net_ns.WebsocketPingMessage, firstMsgType) {
		return
	}

	//check heartbeats
	for i, msgType := range receivedMessageTypes {
		if msgType != net_ns.WebsocketPingMessage {
			continue
		}
		msgPayload := receivedMessagePayloads[i]

		var hearbeat heartbeat
		err = json.Unmarshal(msgPayload, &hearbeat)
		if !assert.NoError(t, err) {
			return
		}

		assert.WithinDuration(t, start.Add(time.Duration(i+1)*HEARTBEAT_INTERVAL), hearbeat.Time, 10*time.Millisecond)
	}
}
