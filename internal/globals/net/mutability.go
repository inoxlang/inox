package internal

func (conn *WebsocketConnection) IsMutable() bool {
	return true
}

func (s *WebsocketServer) IsMutable() bool {
	return true
}

func (conn *TcpConn) IsMutable() bool {
	return true
}
