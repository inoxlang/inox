package ws_ns

import (
	"fmt"
	"time"
)

func (conn *WebsocketConnection) setReadDeadlineNextMessageNoLock() {
	deadline := time.Now().Add(DEFAULT_WAIT_FOR_NEXT_MESSAGE_TIMEOUT)
	fmt.Println("deadline is now", deadline)
	conn.conn.SetReadDeadline(deadline)
}
