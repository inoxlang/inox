package net_ns

import (
	"testing"
	"time"

	core "github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/inoxlang/inox/internal/permkind"
	"github.com/stretchr/testify/assert"
)

func TestWebsocketConnection(t *testing.T) {
	const HTTPS_HOST = core.Host("https://localhost:8080")
	const ENDPOINT = core.URL("wss://localhost:8080/")

	t.Run("connection should be allowed even if the client's context has only a write permission", func(t *testing.T) {
		closeChan := createWebsocketServer(testWebsocketServerConfig{
			host:           HTTPS_HOST,
			messageTimeout: time.Second,
			echo:           true,
		}, nil)
		defer func() {
			go func() {
				closeChan <- struct{}{}
			}()
		}()

		clientCtx := core.NewContext(core.ContextConfig{
			Permissions: []core.Permission{
				core.WebsocketPermission{Kind_: permkind.Write, Endpoint: ENDPOINT},
			},
			Limits:     []core.Limits{{Name: WS_SIMUL_CONN_TOTAL_LIMIT_NAME, Kind: core.TotalLimit, Value: 1}},
			Filesystem: fs_ns.GetOsFilesystem(),
		})

		conn, err := websocketConnect(clientCtx, ENDPOINT, core.Option{Name: "insecure", Value: core.True})
		if !assert.NoError(t, err) {
			return
		}

		//we check that there are no tokens left
		total, err := clientCtx.GetTotal(WS_SIMUL_CONN_TOTAL_LIMIT_NAME)
		assert.NoError(t, err)
		assert.Equal(t, int64(0), total)

		conn.Close()

		//we check that the tokens have been given back
		total, err = clientCtx.GetTotal(WS_SIMUL_CONN_TOTAL_LIMIT_NAME)
		assert.NoError(t, err)
		assert.Equal(t, int64(1), total)

		clientCtx.Take(WS_SIMUL_CONN_TOTAL_LIMIT_NAME, 1)
		total, err = clientCtx.GetTotal(WS_SIMUL_CONN_TOTAL_LIMIT_NAME)
		assert.NoError(t, err)
		assert.Equal(t, int64(0), total)

		//we check that calling close again do no increase the token count
		conn.Close()
		total, err = clientCtx.GetTotal(WS_SIMUL_CONN_TOTAL_LIMIT_NAME)
		assert.NoError(t, err)
		assert.Equal(t, int64(0), total)
	})

	t.Run("manually close connection", func(t *testing.T) {
		closeChan := createWebsocketServer(testWebsocketServerConfig{
			host:           HTTPS_HOST,
			messageTimeout: time.Second,
			echo:           true,
		}, nil)
		defer func() {
			go func() {
				closeChan <- struct{}{}
			}()
		}()

		clientCtx := core.NewContext(core.ContextConfig{
			Permissions: []core.Permission{
				core.WebsocketPermission{Kind_: permkind.Read, Endpoint: ENDPOINT},
			},
			Limits:     []core.Limits{{Name: WS_SIMUL_CONN_TOTAL_LIMIT_NAME, Kind: core.TotalLimit, Value: 1}},
			Filesystem: fs_ns.GetOsFilesystem(),
		})

		conn, err := websocketConnect(clientCtx, ENDPOINT, core.Option{Name: "insecure", Value: core.True})
		if !assert.NoError(t, err) {
			return
		}

		//we check that there are no tokens left
		total, err := clientCtx.GetTotal(WS_SIMUL_CONN_TOTAL_LIMIT_NAME)
		assert.NoError(t, err)
		assert.Equal(t, int64(0), total)

		conn.Close()

		//we check that the tokens have been given back
		total, err = clientCtx.GetTotal(WS_SIMUL_CONN_TOTAL_LIMIT_NAME)
		assert.NoError(t, err)
		assert.Equal(t, int64(1), total)

		clientCtx.Take(WS_SIMUL_CONN_TOTAL_LIMIT_NAME, 1)
		total, err = clientCtx.GetTotal(WS_SIMUL_CONN_TOTAL_LIMIT_NAME)
		assert.NoError(t, err)
		assert.Equal(t, int64(0), total)

		//we check that calling close again do no increase the token count
		conn.Close()
		total, err = clientCtx.GetTotal(WS_SIMUL_CONN_TOTAL_LIMIT_NAME)
		assert.NoError(t, err)
		assert.Equal(t, int64(0), total)
	})

	t.Run("readJSON should return an error on timeout", func(t *testing.T) {
		//we set a very small timeout duration.
		closeChan := createWebsocketServer(testWebsocketServerConfig{
			host:              HTTPS_HOST,
			messageTimeout:    10 * time.Millisecond,
			doNotReadMessages: true,
		}, nil)
		defer func() {
			go func() {
				closeChan <- struct{}{}
			}()
		}()

		clientCtx := core.NewContext(core.ContextConfig{
			Permissions: []core.Permission{
				core.WebsocketPermission{Kind_: permkind.Read, Endpoint: ENDPOINT},
			},
			Limits:     []core.Limits{{Name: WS_SIMUL_CONN_TOTAL_LIMIT_NAME, Kind: core.TotalLimit, Value: 1}},
			Filesystem: fs_ns.GetOsFilesystem(),
		})

		conn, err := websocketConnect(clientCtx, ENDPOINT, core.Option{Name: "insecure", Value: core.True})
		if !assert.NoError(t, err) {
			return
		}

		val, err := conn.readJSON(clientCtx) //timeout
		assert.ErrorContains(t, err, "i/o timeout")
		assert.Nil(t, val)

		//we check that the tokens have been given back
		total, err := clientCtx.GetTotal(WS_SIMUL_CONN_TOTAL_LIMIT_NAME)
		assert.NoError(t, err)
		assert.Equal(t, int64(1), total)

		clientCtx.Take(WS_SIMUL_CONN_TOTAL_LIMIT_NAME, 1)
		total, err = clientCtx.GetTotal(WS_SIMUL_CONN_TOTAL_LIMIT_NAME)
		assert.NoError(t, err)
		assert.Equal(t, int64(0), total)

		//we check that calling close again do no increase the token count
		conn.Close()
		total, err = clientCtx.GetTotal(WS_SIMUL_CONN_TOTAL_LIMIT_NAME)
		assert.NoError(t, err)
		assert.Equal(t, int64(0), total)

	})

	t.Run("ReadMessage should return an error on timeout", func(t *testing.T) {
		// set a very small timeout duration.
		closeChan := createWebsocketServer(testWebsocketServerConfig{
			host:              HTTPS_HOST,
			messageTimeout:    10 * time.Millisecond,
			doNotReadMessages: true,
		}, nil)
		defer func() {
			go func() {
				closeChan <- struct{}{}
			}()
		}()

		clientCtx := core.NewContext(core.ContextConfig{
			Permissions: []core.Permission{
				core.WebsocketPermission{Kind_: permkind.Read, Endpoint: ENDPOINT},
				core.WebsocketPermission{Kind_: permkind.Write, Endpoint: ENDPOINT},
			},
			Limits:     []core.Limits{{Name: WS_SIMUL_CONN_TOTAL_LIMIT_NAME, Kind: core.TotalLimit, Value: 1}},
			Filesystem: fs_ns.GetOsFilesystem(),
		})

		conn, err := websocketConnect(clientCtx, ENDPOINT, core.Option{Name: "insecure", Value: core.True})
		if !assert.NoError(t, err) {
			return
		}

		_, p, err := conn.ReadMessage(clientCtx) //timeout
		assert.ErrorContains(t, err, "i/o timeout")
		assert.Nil(t, p)

		//we check that the tokens have been given back
		total, err := clientCtx.GetTotal(WS_SIMUL_CONN_TOTAL_LIMIT_NAME)
		assert.NoError(t, err)
		assert.Equal(t, int64(1), total)

		clientCtx.Take(WS_SIMUL_CONN_TOTAL_LIMIT_NAME, 1)
		total, err = clientCtx.GetTotal(WS_SIMUL_CONN_TOTAL_LIMIT_NAME)
		assert.NoError(t, err)
		assert.Equal(t, int64(0), total)

		//we check that calling close again do no increase the token count
		conn.Close()
		total, err = clientCtx.GetTotal(WS_SIMUL_CONN_TOTAL_LIMIT_NAME)
		assert.NoError(t, err)
		assert.Equal(t, int64(0), total)
	})
}
