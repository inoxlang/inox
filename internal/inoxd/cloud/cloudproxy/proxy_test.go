package cloudproxy

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/inoxlang/inox/internal/globals/net_ns"
	"github.com/inoxlang/inox/internal/permkind"
	"github.com/inoxlang/inox/internal/project_server"
	"github.com/stretchr/testify/assert"
)

func TestCloudProxy(t *testing.T) {
	t.Skip("manual test")
	username := "<Github username>"

	//outW := io.Discard
	//errW := io.Discard
	outW := os.Stdout
	errW := os.Stdout

	config := CloudProxyConfig{
		AnonymousAccountDatabasePath: "/db.kv",
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

	host := core.Host("wss://localhost:" + project_server.DEFAULT_PROJECT_SERVER_PORT)

	ctx := core.NewContexWithEmptyState(core.ContextConfig{
		Permissions: []core.Permission{
			core.WebsocketPermission{Kind_: permkind.Read, Endpoint: host},
			core.WebsocketPermission{Kind_: permkind.Write, Endpoint: host},
		},
		Limits:              []core.Limit{{Name: net_ns.WS_SIMUL_CONN_TOTAL_LIMIT_NAME, Kind: core.TotalLimit, Value: 5}},
		ParentStdLibContext: goctx,
	}, nil)

	insecure := true
	registrationEndpointWithQuery := host.URLWithPath(ACCOUNT_REGISTRATION_URL_PATH) + "?" + ACCOUNT_REGISTRATION_HOSTER_PARAM_NAME + "=Github"

	var accountToken string

	//register account
	{
		socket, err := net_ns.WebsocketConnect(ctx, registrationEndpointWithQuery, insecure, nil)

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
		if !assert.NoError(t, socket.WriteMessage(ctx, net_ns.WebsocketTextMessage, []byte(username))) {
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
		assert.NoError(t, socket.WriteMessage(ctx, net_ns.WebsocketTextMessage, []byte("ack:token")))

		//wait for the database to persist the account.
		time.Sleep(time.Second)
	}

	{
		//connection without account token should not work
		socket, err := net_ns.WebsocketConnect(ctx, host.URLWithPath("/"), insecure, nil)

		if !assert.Error(t, err) {
			socket.Close()
			return
		}
	}

	{
		//connection with the account token should work
		socket, err := net_ns.WebsocketConnect(ctx, host.URLWithPath("/"), insecure, http.Header{
			ACCOUNT_TOKEN_HEADER_NAME: []string{accountToken},
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
