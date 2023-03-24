package internal

import (
	"fmt"
	"io"
)

func (kvs *LocalDatabase) PrettyPrint(w io.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) (int, error) {
	return fmt.Fprintf(w, "%T(...)", kvs)
}
