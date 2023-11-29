package net_ns

import (
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/prettyprint"
	pprint "github.com/inoxlang/inox/internal/prettyprint"
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
	case "sendJSON":
		return symbolic.WrapGoMethod(conn.sendJSON), true
	case "readJSON":
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
	return []string{"sendJSON", "readJSON", "close"}
}

func (conn *WebsocketConnection) sendJSON(ctx *symbolic.Context, msg symbolic.Value) *symbolic.Error {
	return nil
}

func (conn *WebsocketConnection) readJSON(ctx *symbolic.Context) (symbolic.Value, *symbolic.Error) {
	return &symbolic.Any{}, nil
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
