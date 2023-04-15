package internal

import (
	"testing"

	core "github.com/inoxlang/inox/internal/core"
	"github.com/stretchr/testify/assert"
)

func TestWebsocketConnection(t *testing.T) {
	const HTTPS_HOST = core.Host("https://localhost:8080")
	const ENDPOINT = URL("wss://localhost:8080/")

	t.Run("manually close connection", func(t *testing.T) {
		closeChan := createTestWebsocketServer(HTTPS_HOST, nil)
		defer func() {
			closeChan <- struct{}{}
		}()

		clientCtx := core.NewContext(ContextConfig{
			Permissions: []Permission{
				WebsocketPermission{Kind_: core.ReadPerm, Endpoint: ENDPOINT},
			},
			Limitations: []Limitation{{Name: WS_SIMUL_CONN_TOTAL_LIMIT_NAME, Kind: core.TotalLimitation, Value: 1}},
		})

		conn, err := websocketConnect(clientCtx, ENDPOINT, core.Option{Name: "insecure", Value: core.True})
		if !assert.NoError(t, err) {
			return
		}

		//we check that there are no tokens left
		total, err := clientCtx.GetTotal(WS_SIMUL_CONN_TOTAL_LIMIT_NAME)
		assert.NoError(t, err)
		assert.Equal(t, int64(0), total)

		conn.close(clientCtx)

		//we check that the tokens have been given back
		total, err = clientCtx.GetTotal(WS_SIMUL_CONN_TOTAL_LIMIT_NAME)
		assert.NoError(t, err)
		assert.Equal(t, int64(1), total)

		clientCtx.Take(WS_SIMUL_CONN_TOTAL_LIMIT_NAME, 1)
		total, err = clientCtx.GetTotal(WS_SIMUL_CONN_TOTAL_LIMIT_NAME)
		assert.NoError(t, err)
		assert.Equal(t, int64(0), total)

		//we check that calling Close again do no increase the token count
		conn.close(clientCtx)
		total, err = clientCtx.GetTotal(WS_SIMUL_CONN_TOTAL_LIMIT_NAME)
		assert.NoError(t, err)
		assert.Equal(t, int64(0), total)
	})

}
