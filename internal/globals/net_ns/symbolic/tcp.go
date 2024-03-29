package net_ns

import (
	"github.com/inoxlang/inox/internal/core/symbolic"
	pprint "github.com/inoxlang/inox/internal/prettyprint"
)

var (
	TCP_CONN_PROPNAMES = []string{"read", "write", "close"}
)

type TcpConn struct {
	symbolic.UnassignablePropsMixin
	_ int
}

func (r *TcpConn) Test(v symbolic.Value, state symbolic.RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	_, ok := v.(*TcpConn)
	return ok
}

func (conn *TcpConn) GetGoMethod(name string) (*symbolic.GoFunction, bool) {
	switch name {
	case "read":
		return symbolic.WrapGoMethod(conn.read), true
	case "write":
		return symbolic.WrapGoMethod(conn.write), true
	case "close":
		return symbolic.WrapGoMethod(conn.close), true
	}
	return nil, false
}

func (conn *TcpConn) Prop(name string) symbolic.Value {
	return symbolic.GetGoMethodOrPanic(name, conn)
}

func (*TcpConn) PropertyNames() []string {
	return TCP_CONN_PROPNAMES
}

func (conn *TcpConn) read(ctx *symbolic.Context) (*symbolic.ByteSlice, *symbolic.Error) {
	return &symbolic.ByteSlice{}, nil
}

func (conn *TcpConn) write(ctx *symbolic.Context, data symbolic.Readable) *symbolic.Error {
	return nil
}

func (conn *TcpConn) close(ctx *symbolic.Context) {
}

func (r *TcpConn) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("tcp-conn")
}

func (r *TcpConn) WidestOfType() symbolic.Value {
	return &TcpConn{}
}
