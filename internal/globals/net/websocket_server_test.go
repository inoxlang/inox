package internal

import (
	"log"
	"os"
	"runtime/debug"
	"testing"
	"time"

	core "github.com/inoxlang/inox/internal/core"
	_fs "github.com/inoxlang/inox/internal/globals/fs"
	_http "github.com/inoxlang/inox/internal/globals/http"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

func createTestWebsocketServer(host core.Host, ctx *Context) (closeChan chan struct{}) {
	if ctx == nil {
		ctx = core.NewContext(core.ContextConfig{
			Permissions: []core.Permission{
				core.WebsocketPermission{Kind_: core.ProvidePerm},
				core.HttpPermission{Kind_: core.ProvidePerm, Entity: host},
			},
			Filesystem: _fs.GetOsFilesystem(),
		})
		serverState := core.NewGlobalState(ctx)
		serverState.Logger = zerolog.New(os.Stdout)
		serverState.Out = os.Stdout
	}

	closeChan = make(chan struct{})

	log.Println(ctx.GetFileSystem(), string(debug.Stack()))

	go func() {
		wsServer, _ := NewWebsocketServer(ctx)
		handler := core.WrapGoFunction(func(ctx *Context, rw *_http.HttpResponseWriter, req *_http.HttpRequest) {
			conn, err := wsServer.Upgrade(rw, req)
			if err != nil {
				log.Panicln("failed to upgrade", err)
			}

			go func() {
				// echo
				var v Value
				var err error
				for ; err == nil; v, err = conn.readJSON(ctx) {
					conn.sendJSON(ctx, v)
				}
			}()
		})

		log.Println(ctx.GetFileSystem(), string(debug.Stack()))

		server, err := _http.NewHttpServer(ctx, host, handler)
		if err != nil {
			log.Panicln("failed to create test server", err)
		}

		select {
		case <-closeChan:
			server.Close(ctx)
		case <-time.After(10 * time.Second):
			server.Close(ctx)
		}
	}()

	time.Sleep(time.Second / 10)

	return
}

func TestWebsocketServer(t *testing.T) {
	t.Run("create with required permission", func(t *testing.T) {
		ctx := core.NewContext(core.ContextConfig{
			Permissions: []core.Permission{
				core.WebsocketPermission{Kind_: core.ProvidePerm},
			},
			Filesystem: _fs.GetOsFilesystem(),
		})
		server, err := NewWebsocketServer(ctx)
		assert.NoError(t, err)
		assert.NotNil(t, server)
	})

	t.Run("create without required permission", func(t *testing.T) {
		ctx := core.NewContext(core.ContextConfig{})
		server, err := NewWebsocketServer(ctx)
		assert.ErrorIs(t, err, core.NewNotAllowedError(core.WebsocketPermission{Kind_: core.ProvidePerm}))
		assert.Nil(t, server)
	})

	t.Run("upgrade", func(t *testing.T) {
		HOST := core.Host("https://localhost:8080")
		ENDPOINT := core.URL("wss://localhost:8080/")

		clientCtx := core.NewContext(core.ContextConfig{
			Permissions: []core.Permission{
				core.WebsocketPermission{Kind_: core.ReadPerm, Endpoint: ENDPOINT},
			},
			Filesystem: _fs.GetOsFilesystem(),
		})

		serverCtx := core.NewContext(core.ContextConfig{
			Permissions: []core.Permission{
				core.WebsocketPermission{Kind_: core.ProvidePerm},
				core.HttpPermission{
					Kind_:  core.ProvidePerm,
					Entity: HOST,
				},
			},
			Filesystem: _fs.GetOsFilesystem(),
		})

		serverState := core.NewGlobalState(serverCtx)
		serverState.Logger = zerolog.New(os.Stdout)
		serverState.Out = os.Stdout

		closeChan := createTestWebsocketServer(HOST, serverCtx)
		defer func() {
			closeChan <- struct{}{}
		}()

		conn, err := websocketConnect(clientCtx, ENDPOINT, core.Option{Name: "insecure", Value: core.True})
		assert.NoError(t, err)
		assert.NotNil(t, conn)
		assert.Equal(t, ENDPOINT, conn.endpoint)
	})

}
