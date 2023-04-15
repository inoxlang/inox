package internal

import (
	net_symbolic "github.com/inoxlang/inox/internal/globals/net/symbolic"
)

func init() {
}

func (conn *WebsocketConnection) ToSymbolicValue(wide bool, encountered map[uintptr]SymbolicValue) (SymbolicValue, error) {
	return &net_symbolic.WebsocketConnection{}, nil
}

func (s *WebsocketServer) ToSymbolicValue(wide bool, encountered map[uintptr]SymbolicValue) (SymbolicValue, error) {
	return &net_symbolic.WebsocketServer{}, nil
}

func (conn *TcpConn) ToSymbolicValue(wide bool, encountered map[uintptr]SymbolicValue) (SymbolicValue, error) {
	return &net_symbolic.TcpConn{}, nil
}
