package net_ns

import core "github.com/inoxlang/inox/internal/core"

func (conn *WebsocketConnection) Equal(ctx *core.Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherConn, ok := other.(*WebsocketConnection)
	return ok && conn == otherConn
}

func (s *WebsocketServer) Equal(ctx *core.Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherServer, ok := other.(*WebsocketServer)
	return ok && s == otherServer
}

func (conn *TcpConn) Equal(ctx *core.Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherConn, ok := other.(*TcpConn)
	return ok && conn == otherConn
}
