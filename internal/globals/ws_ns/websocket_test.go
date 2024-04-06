package ws_ns

import (
	"bytes"
	"testing"
	"time"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/permbase"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/stretchr/testify/assert"
)

func TestWebsocketConnection(t *testing.T) {
	if !core.AreDefaultRequestHandlingLimitsSet() {
		core.SetDefaultRequestHandlingLimits([]core.Limit{})
		defer func() {
			core.UnsetDefaultRequestHandlingLimits()
		}()
	}

	if !core.AreDefaultMaxRequestHandlerLimitsSet() {
		core.SetDefaultMaxRequestHandlerLimits([]core.Limit{})
		defer func() {
			core.UnsetDefaultMaxRequestHandlerLimits()
		}()
	}

	t.Run("connection should be allowed even if the client's context has only a write permission", func(t *testing.T) {

		HTTPS_HOST, ENDPOINT := getNextHostAndEndpoint()

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
				core.WebsocketPermission{Kind_: permbase.Write, Endpoint: ENDPOINT},
			},
			Limits:     []core.Limit{{Name: WS_SIMUL_CONN_TOTAL_LIMIT_NAME, Kind: core.TotalLimit, Value: 1}},
			Filesystem: fs_ns.GetOsFilesystem(),
		})
		defer clientCtx.CancelGracefully()

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

		HTTPS_HOST, ENDPOINT := getNextHostAndEndpoint()

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
				core.WebsocketPermission{Kind_: permbase.Read, Endpoint: ENDPOINT},
			},
			Limits:     []core.Limit{{Name: WS_SIMUL_CONN_TOTAL_LIMIT_NAME, Kind: core.TotalLimit, Value: 1}},
			Filesystem: fs_ns.GetOsFilesystem(),
		})
		defer clientCtx.CancelGracefully()

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
		t.Skip("TO FIX")

		HTTPS_HOST, ENDPOINT := getNextHostAndEndpoint()

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
				core.WebsocketPermission{Kind_: permbase.Read, Endpoint: ENDPOINT},
			},
			Limits:     []core.Limit{{Name: WS_SIMUL_CONN_TOTAL_LIMIT_NAME, Kind: core.TotalLimit, Value: 1}},
			Filesystem: fs_ns.GetOsFilesystem(),
		})
		defer clientCtx.CancelGracefully()

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
		t.Skip("TO FIX")

		HTTPS_HOST, ENDPOINT := getNextHostAndEndpoint()

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
				core.WebsocketPermission{Kind_: permbase.Read, Endpoint: ENDPOINT},
				core.WebsocketPermission{Kind_: permbase.Write, Endpoint: ENDPOINT},
			},
			Limits:     []core.Limit{{Name: WS_SIMUL_CONN_TOTAL_LIMIT_NAME, Kind: core.TotalLimit, Value: 1}},
			Filesystem: fs_ns.GetOsFilesystem(),
		})
		defer clientCtx.CancelGracefully()

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

	t.Run("StartReadingAllMessagesIntoChan", func(t *testing.T) {

		setup := func() (*WebsocketConnection, *core.Context, chan struct{}, bool) {
			HTTPS_HOST, ENDPOINT := getNextHostAndEndpoint()

			closeChan := createWebsocketServer(testWebsocketServerConfig{
				host:           HTTPS_HOST,
				messageTimeout: time.Second,
				echo:           true,
			}, nil)

			clientCtx := core.NewContext(core.ContextConfig{
				Permissions: []core.Permission{
					core.WebsocketPermission{Kind_: permbase.Read, Endpoint: ENDPOINT},
					core.WebsocketPermission{Kind_: permbase.Write, Endpoint: ENDPOINT},
				},
				Limits:     []core.Limit{{Name: WS_SIMUL_CONN_TOTAL_LIMIT_NAME, Kind: core.TotalLimit, Value: 1}},
				Filesystem: fs_ns.GetOsFilesystem(),
			})

			conn, err := websocketConnect(clientCtx, ENDPOINT, core.Option{Name: "insecure", Value: core.True})
			if !assert.NoError(t, err) {
				clientCtx.CancelGracefully()
				return nil, nil, nil, false
			}

			return conn, clientCtx, closeChan, true
		}

		t.Run("base case", func(t *testing.T) {
			conn, ctx, closeServerChan, ok := setup()
			if !ok {
				return
			}
			defer ctx.CancelGracefully()
			defer func() {
				closeServerChan <- struct{}{}
			}()

			channel := make(chan WebsocketMessageChanItem, 10)
			if !assert.NoError(t, conn.StartReadingAllMessagesIntoChan(ctx, channel)) {
				return
			}

			payload := `"a"`

			if !assert.NoError(t, conn.WriteMessage(ctx, WebsocketTextMessage, []byte(payload))) {
				return
			}

			select {
			//echo
			case item := <-channel:
				item.Payload = bytes.TrimSpace(item.Payload)

				assert.Equal(t, WebsocketMessageChanItem{
					Type:    WebsocketTextMessage,
					Payload: []byte(payload),
				}, item)
			case <-time.After(100 * time.Millisecond):
				t.Log("timeout")
				t.Fail()
				return
			}

			//close connection
			conn.Close()

			select {
			case item := <-channel:
				assert.Nil(t, item.Payload)
				assert.Zero(t, item.Type)
				assert.Error(t, item.Error)
			case <-time.After(100 * time.Millisecond):
				t.Log("timeout")
				t.Fail()
				return
			}
		})

		t.Run("a second call should return ErrAlreadyReadingAllMessages and should not have any effect", func(t *testing.T) {
			conn, ctx, closeServerChan, ok := setup()
			if !ok {
				return
			}
			defer ctx.CancelGracefully()
			defer func() {
				closeServerChan <- struct{}{}
			}()

			channel := make(chan WebsocketMessageChanItem, 10)
			if !assert.NoError(t, conn.StartReadingAllMessagesIntoChan(ctx, channel)) {
				return
			}

			//second call
			if !assert.ErrorIs(t, conn.StartReadingAllMessagesIntoChan(ctx, channel), ErrAlreadyReadingAllMessages) {
				return
			}
			//the second call should have no effect on the current reading.

			payload := `"a"`

			if !assert.NoError(t, conn.WriteMessage(ctx, WebsocketTextMessage, []byte(payload))) {
				return
			}

			select {
			//echo
			case item := <-channel:
				item.Payload = bytes.TrimSpace(item.Payload)

				assert.Equal(t, WebsocketMessageChanItem{
					Type:    WebsocketTextMessage,
					Payload: []byte(payload),
				}, item)
			case <-time.After(100 * time.Millisecond):
				t.Log("timeout")
				t.Fail()
				return
			}

			//close connection
			conn.Close()

			select {
			case item := <-channel:
				assert.Nil(t, item.Payload)
				assert.Zero(t, item.Type)
				assert.Error(t, item.Error)
			case <-time.After(100 * time.Millisecond):
				t.Log("timeout")
				t.Fail()
				return
			}
		})
	})

}
