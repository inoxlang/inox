package internal

import (
	"fmt"
	"io"
)

func (conn *WebsocketConnection) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "%#v", conn)
}

func (s *WebsocketServer) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "%#v", s)
}

func (conn *TcpConn) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "%#v", conn)
}
