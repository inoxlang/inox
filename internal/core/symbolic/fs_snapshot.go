package symbolic

import (
	"bufio"

	pprint "github.com/inoxlang/inox/internal/pretty_print"
	"github.com/inoxlang/inox/internal/utils"
)

var (
	ANY_FS_SNAPSHOT_IL = &FilesystemSnapshotIL{}
)

// A FilesystemSnapshotIL represents a symbolic FilesystemSnapshotIL we don't the concrete type.
type FilesystemSnapshotIL struct {
	SerializableMixin
}

func (*FilesystemSnapshotIL) _fssnapshot() {

}

func (t *FilesystemSnapshotIL) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	switch v.(type) {
	case *FilesystemSnapshotIL:
		return true
	default:
		return false
	}
}

func (t *FilesystemSnapshotIL) WidestOfType() Value {
	return ANY_FS_SNAPSHOT_IL
}

func (t *FilesystemSnapshotIL) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%fs-snapshot")))
}
