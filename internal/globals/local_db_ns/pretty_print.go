package local_db_ns

import (
	"bufio"
	"fmt"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/utils"
)

func (kvs *LocalDatabase) PrettyPrint(w *bufio.Writer, config *core.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(fmt.Fprintf(w, "%T(...)", kvs))
}
