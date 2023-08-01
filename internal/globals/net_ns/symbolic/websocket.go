package net_ns

import (
	"bufio"

	"github.com/inoxlang/inox/internal/core/symbolic"
	pprint "github.com/inoxlang/inox/internal/pretty_print"
	"github.com/inoxlang/inox/internal/utils"
)

type WebsocketConnection struct {
	symbolic.UnassignablePropsMixin
	_ int
}

func (r *WebsocketConnection) Test(v symbolic.SymbolicValue) bool {
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

func (r *WebsocketConnection) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%websocket-conn")))
}

func (r *WebsocketConnection) WidestOfType() symbolic.SymbolicValue {
	return &WebsocketConnection{}
}
