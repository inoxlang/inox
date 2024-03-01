package ws_ns

import (
	"github.com/inoxlang/inox/internal/core"

	ws_symbolic "github.com/inoxlang/inox/internal/globals/ws_ns/symbolic"
)

//core.Value implementation for WebsocketConnection.

func (conn *WebsocketConnection) GetGoMethod(name string) (*core.GoFunction, bool) {
	switch name {
	case "send_json":
		return core.WrapGoMethod(conn.sendJSON), true
	case "read_json":
		return core.WrapGoMethod(conn.readJSON), true
	case "close":
		return core.WrapGoMethod(conn.Close), true
	}
	return nil, false
}

func (conn *WebsocketConnection) Prop(ctx *core.Context, name string) core.Value {
	method, ok := conn.GetGoMethod(name)
	if !ok {
		panic(core.FormatErrPropertyDoesNotExist(name, conn))
	}
	return method
}

func (*WebsocketConnection) SetProp(ctx *core.Context, name string, value core.Value) error {
	return core.ErrCannotSetProp
}

func (*WebsocketConnection) PropertyNames(ctx *core.Context) []string {
	return ws_symbolic.WEBSOCKET_PROPNAMES
}
