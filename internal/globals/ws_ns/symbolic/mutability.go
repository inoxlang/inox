package net_ns

func (conn *WebsocketConnection) IsMutable() bool {
	return true
}

func (s *WebsocketServer) IsMutable() bool {
	return true
}
