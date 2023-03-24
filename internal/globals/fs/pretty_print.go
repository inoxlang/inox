package internal

import (
	"fmt"
	"io"

	core "github.com/inox-project/inox/internal/core"
)

func (f *File) PrettyPrint(w io.Writer, config *core.PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "%#v", f)
}

func (evs *FilesystemEventSource) PrettyPrint(w io.Writer, config *core.PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "%#v", evs)
}
