package net_ns

import (
	"bufio"

	"github.com/inoxlang/inox/internal/core/symbolic"
	pprint "github.com/inoxlang/inox/internal/pretty_print"
	"github.com/inoxlang/inox/internal/utils"
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
	return []string{"read", "write", "close"}
}

func (conn *TcpConn) read(ctx *symbolic.Context) (*symbolic.ByteSlice, *symbolic.Error) {
	return &symbolic.ByteSlice{}, nil
}

func (conn *TcpConn) write(ctx *symbolic.Context, data symbolic.Readable) *symbolic.Error {
	return nil
}

func (conn *TcpConn) close(ctx *symbolic.Context) {
}

func (r *TcpConn) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%tcp-conn")))
}

func (r *TcpConn) WidestOfType() symbolic.Value {
	return &TcpConn{}
}
