package internal

import (
	"fmt"
	"io"

	core "github.com/inoxlang/inox/internal/core"
)

func (n *HTMLNode) PrettyPrint(w io.Writer, config *core.PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "%#v", n)
}
