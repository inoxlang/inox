package ws_ns

import (
	"bufio"
	"fmt"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/utils"
)

func (conn *WebsocketConnection) PrettyPrint(w *bufio.Writer, config *core.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(fmt.Fprintf(w, "%#v", conn))
}

func (s *WebsocketServer) PrettyPrint(w *bufio.Writer, config *core.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(fmt.Fprintf(w, "%#v", s))
}
