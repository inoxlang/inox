package symbolic

import (
	pprint "github.com/inoxlang/inox/internal/prettyprint"
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

func (t *FilesystemSnapshotIL) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("fs-snapshot")
}
