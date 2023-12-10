package net_ns

import (
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/symbolic"
	net_symbolic "github.com/inoxlang/inox/internal/globals/net_ns/symbolic"
)

func init() {
}

func (conn *TcpConn) ToSymbolicValue(ctx *core.Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return &net_symbolic.TcpConn{}, nil
}
