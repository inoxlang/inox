package internal

import (
	"bufio"
	"fmt"

	"github.com/inoxlang/inox/internal/utils"
)

func (kvs *LocalDatabase) PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(fmt.Fprintf(w, "%T(...)", kvs))
}
