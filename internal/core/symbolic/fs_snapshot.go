package symbolic

import (
	"bufio"

	pprint "github.com/inoxlang/inox/internal/pretty_print"
	"github.com/inoxlang/inox/internal/utils"
)

var (
	ANY_FS_SNAPSHOT = &AnyFilesystemSnapshot{}
	_               = FilesystemSnapshot(ANY_FS_SNAPSHOT)
)

type FilesystemSnapshot interface {
	_fssnapshot()
}

// A AnyFilesystemSnapshot represents a symbolic AnyFilesystemSnapshot we don't the concrete type.
type AnyFilesystemSnapshot struct {
}

func (*AnyFilesystemSnapshot) _fssnapshot() {

}

func (t *AnyFilesystemSnapshot) Test(v SymbolicValue) bool {
	switch v.(type) {
	case *AnyFilesystemSnapshot:
		return true
	default:
		return false
	}
}

func (t *AnyFilesystemSnapshot) WidestOfType() SymbolicValue {
	return ANY_LTHREAD
}

func (t *AnyFilesystemSnapshot) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%fs-snapshot")))
}
