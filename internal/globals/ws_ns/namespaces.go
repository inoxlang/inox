package ws_ns

import (
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/symbolic"
	ws_symbolic "github.com/inoxlang/inox/internal/globals/ws_ns/symbolic"
)

func init() {
	// register limits
	core.RegisterLimit(WS_SIMUL_CONN_TOTAL_LIMIT_NAME, core.TotalLimit, 0)

	// register symbolic version of Go Functions
	core.RegisterSymbolicGoFunctions([]any{
		websocketConnect, func(ctx *symbolic.Context, u *symbolic.URL, opts ...*symbolic.Option) (*ws_symbolic.WebsocketConnection, *symbolic.Error) {
			return &ws_symbolic.WebsocketConnection{}, nil
		},
		NewWebsocketServer, func(ctx *symbolic.Context) (*ws_symbolic.WebsocketServer, *symbolic.Error) {
			return &ws_symbolic.WebsocketServer{}, nil
		},
	})

}

func NewWebsocketNamespace() *core.Namespace {
	return core.NewNamespace("ws", map[string]core.Value{
		"connect": core.ValOf(websocketConnect),
		"Server":  core.ValOf(NewWebsocketServer),
	})
}
