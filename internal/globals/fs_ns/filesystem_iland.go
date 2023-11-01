package fs_ns

import (
	"github.com/inoxlang/inox/internal/afs"
	"github.com/inoxlang/inox/internal/core"
)

var (
	_ core.Value = (*FilesystemIL)(nil)
)

type FilesystemIL struct {
	afs.Filesystem
}

func NewMemFilesystemIL(maxTotalStorageSize core.ByteCount) *FilesystemIL {
	return &FilesystemIL{
		Filesystem: NewMemFilesystem(maxTotalStorageSize),
	}
}
