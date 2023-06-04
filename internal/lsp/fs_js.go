//go:build js

package internal

import (
	afs "github.com/inoxlang/inox/internal/afs"
	fs_ns "github.com/inoxlang/inox/internal/globals/fs_ns"
)

func NewDefaultFilesystem() *Filesystem {
	return &Filesystem{
		Filesystem:       fs_ns.NewMemFilesystem(DEFAULT_MAX_IN_MEM_FS_STORAGE_SIZE),
		unsavedDocuments: nil,
	}
}

func (fs *Filesystem) docsFS() afs.Filesystem {
	return fs
}
