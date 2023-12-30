package net_ns

import (
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/prettyprint"
	pprint "github.com/inoxlang/inox/internal/prettyprint"
)

var (
	WEBSOCKET_PROPNAMES = []string{"send_json", "read_json", "close"}
)

type WebsocketConnection struct {
	symbolic.UnassignablePropsMixin
	_ int
}

func (r *WebsocketConnection) Test(v symbolic.Value, state symbolic.RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	_, ok := v.(*WebsocketConnection)
	return ok
}

func (conn *WebsocketConnection) GetGoMethod(name string) (*symbolic.GoFunction, bool) {
	switch name {
	case "send_json":
		return symbolic.WrapGoMethod(conn.sendJSON), true
	case "read_json":
		return symbolic.WrapGoMethod(conn.readJSON), true
	case "close":
		return symbolic.WrapGoMethod(conn.close), true
	}
	return nil, false
}

func (conn *WebsocketConnection) Prop(name string) symbolic.Value {
	return symbolic.GetGoMethodOrPanic(name, conn)
}

func (*WebsocketConnection) PropertyNames() []string {
	return WEBSOCKET_PROPNAMES
}

func (conn *WebsocketConnection) sendJSON(ctx *symbolic.Context, msg symbolic.Value) *symbolic.Error {
	return nil
}

func (conn *WebsocketConnection) readJSON(ctx *symbolic.Context) (symbolic.Value, *symbolic.Error) {
	return symbolic.ANY, nil
}

func (conn *WebsocketConnection) close(ctx *symbolic.Context) *symbolic.Error {
	return nil
}

func (r *WebsocketConnection) PrettyPrint(w prettyprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("websocket-conn")
}

func (r *WebsocketConnection) WidestOfType() symbolic.Value {
	return &WebsocketConnection{}
}
