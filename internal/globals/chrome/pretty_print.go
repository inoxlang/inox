package internal

import (
	"fmt"
	"io"

	core "github.com/inox-project/inox/internal/core"
)

func (h *Handle) PrettyPrint(w io.Writer, config *core.PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "%T(...)", h)
}
