package ws_ns

import (
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/symbolic"
	net_symbolic "github.com/inoxlang/inox/internal/globals/net_ns/symbolic"
)

func init() {
	// register limits
	core.RegisterLimit(WS_SIMUL_CONN_TOTAL_LIMIT_NAME, core.TotalLimit, 0)

	// register symbolic version of Go Functions
	core.RegisterSymbolicGoFunctions([]any{
		websocketConnect, func(ctx *symbolic.Context, u *symbolic.URL, opts ...*symbolic.Option) (*net_symbolic.WebsocketConnection, *symbolic.Error) {
			return &net_symbolic.WebsocketConnection{}, nil
		},
		NewWebsocketServer, func(ctx *symbolic.Context) (*net_symbolic.WebsocketServer, *symbolic.Error) {
			return &net_symbolic.WebsocketServer{}, nil
		},
	})

}

func NewWebsocketNamespace() *core.Namespace {
	return core.NewNamespace("ws", map[string]core.Value{
		"connect": core.ValOf(websocketConnect),
		"Server":  core.ValOf(NewWebsocketServer),
	})
}
