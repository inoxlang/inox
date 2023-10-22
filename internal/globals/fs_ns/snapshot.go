package fs_ns

import (
	"reflect"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/symbolic"
)

const (
	FS_SNAPSHOT_SYMB_DESC_FILES_PROPNAME = "files"
)

var (
	NEW_FS_SNAPSHOT_DESC = symbolic.NewInexactObject2(map[string]symbolic.Serializable{
		FS_SNAPSHOT_SYMB_DESC_FILES_PROPNAME: symbolic.ANY_DICT,
	})

	NEW_FS_SNAPSHOT_SYMB_ARGS      = &[]symbolic.Value{NEW_FS_SNAPSHOT_DESC}
	NEW_FS_SNAPSHOT_SYMB_ARG_NAMES = []string{"description"}
)

func NewFilesystemSnapshot(ctx *core.Context, desc *core.Object) *core.FilesystemSnapshotIL {

	var snapshot core.FilesystemSnapshot

	desc.ForEachEntry(func(k string, v core.Serializable) error {
		switch k {
		case FS_SNAPSHOT_SYMB_DESC_FILES_PROPNAME:
			dict := v.(*core.Dictionary)

			fls := NewMemFilesystem(TRUE_MAX_IN_MEM_STORAGE)

			err := makeFileHierarchy(ctx, makeFileHieararchyParams{
				fls:     fls,
				key:     "/",
				content: dict,
				depth:   0,
			})

			if err != nil {
				panic(err)
			}

			snapshot, err = fls.TakeFilesystemSnapshot(func(ChecksumSHA256 [32]byte) core.AddressableContent {
				return nil
			})

			if err != nil {
				panic(err)
			}

			return nil
		}
		return nil
	})

	if reflect.ValueOf(snapshot).IsNil() {
		panic(core.ErrUnreachable)
	}

	return core.WrapFsSnapshot(snapshot)
}
