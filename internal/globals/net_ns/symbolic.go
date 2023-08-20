package net_ns

import (
	core "github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/symbolic"
	net_symbolic "github.com/inoxlang/inox/internal/globals/net_ns/symbolic"
)

func init() {
}

func (conn *WebsocketConnection) ToSymbolicValue(ctx *core.Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &net_symbolic.WebsocketConnection{}, nil
}

func (s *WebsocketServer) ToSymbolicValue(ctx *core.Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &net_symbolic.WebsocketServer{}, nil
}

func (conn *TcpConn) ToSymbolicValue(ctx *core.Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &net_symbolic.TcpConn{}, nil
}
