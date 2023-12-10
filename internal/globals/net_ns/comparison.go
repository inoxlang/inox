package net_ns

import "github.com/inoxlang/inox/internal/core"

func (conn *TcpConn) Equal(ctx *core.Context, other core.Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherConn, ok := other.(*TcpConn)
	return ok && conn == otherConn
}
