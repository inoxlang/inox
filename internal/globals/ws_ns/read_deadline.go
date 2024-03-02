package ws_ns

import (
	"time"
)

func (conn *WebsocketConnection) setReadDeadlineNextMessageNoLock() {
	deadline := time.Now().Add(DEFAULT_WAIT_FOR_NEXT_MESSAGE_TIMEOUT)
	conn.conn.SetReadDeadline(deadline)
}
