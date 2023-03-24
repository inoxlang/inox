package internal

import (
	symbolic "github.com/inox-project/inox/internal/core/symbolic"
)

type WebsocketConnection struct {
	symbolic.UnassignablePropsMixin
	_ int
}

func (r *WebsocketConnection) Test(v symbolic.SymbolicValue) bool {
	_, ok := v.(*WebsocketConnection)
	return ok
}

func (r WebsocketConnection) Clone(clones map[uintptr]symbolic.SymbolicValue) symbolic.SymbolicValue {
	return &WebsocketConnection{}
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
	return &symbolic.GoFunction{}, false
}

func (conn *WebsocketConnection) Prop(name string) symbolic.SymbolicValue {
	return symbolic.GetGoMethodOrPanic(name, conn)
}

func (*WebsocketConnection) PropertyNames() []string {
	return []string{"sendJSON", "readJSON", "close"}
}

func (conn *WebsocketConnection) sendJSON(ctx *symbolic.Context, msg symbolic.SymbolicValue) *symbolic.Error {
	return nil
}

func (conn *WebsocketConnection) readJSON(ctx *symbolic.Context) (symbolic.SymbolicValue, *symbolic.Error) {
	return &symbolic.Any{}, nil
}

func (conn *WebsocketConnection) close(ctx *symbolic.Context) *symbolic.Error {
	return nil
}

func (r *WebsocketConnection) Widen() (symbolic.SymbolicValue, bool) {
	return nil, false
}

func (a *WebsocketConnection) IsWidenable() bool {
	return false
}

func (r *WebsocketConnection) String() string {
	return "websocket-conn"
}

func (r *WebsocketConnection) WidestOfType() symbolic.SymbolicValue {
	return &WebsocketConnection{}
}
