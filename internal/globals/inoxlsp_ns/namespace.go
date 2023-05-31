package inoxlsp_ns

import (
	"fmt"

	core "github.com/inoxlang/inox/internal/core"
	symbolic "github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/lsp/jsonrpc"

	lsp "github.com/inoxlang/inox/internal/lsp"
)

func init() {
	core.RegisterSymbolicGoFunction(StartLspServer, func(ctx *symbolic.Context, host *symbolic.Host, fn *symbolic.InoxFunction) *symbolic.Error {
		return nil
	})
}

func NewInoxLspNamespace() *core.Record {
	return core.NewRecordFromMap(core.ValMap{
		"start_websocket_server": core.WrapGoFunction(StartLspServer),
	})
}

func StartLspServer(ctx *core.Context, host core.Host, fn *core.InoxFunction) error {
	state := ctx.GetClosestState()
	childCtx := ctx.BoundChild()

	return lsp.StartLSPServer(childCtx, lsp.LSPServerOptions{
		Filesystem: lsp.NewFilesystem(state.Ctx.GetFileSystem(), nil),
		Websocket: &lsp.WebsocketOptions{
			Addr: host.WithoutScheme(),
		},
		UseContextLogger: true,
		OnSession: func(s *jsonrpc.Session) error {
			fmt.Println("!!!!!!", s)
			_ = state
			return nil
		},
	})
}
