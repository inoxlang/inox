package internal

import (
	afs "github.com/inoxlang/inox/internal/afs"
)

const (
	DEFAULT_MAX_IN_MEM_FS_STORAGE_SIZE = 10_000_000
)

// Filesystem is a filesystem that stores the unsaved documents in a separate filesystem.
type Filesystem struct {
	afs.Filesystem
	unsavedDocuments afs.Filesystem
}


func NewFilesystem(base afs.Filesystem, unsavedDocumentFs afs.Filesystem) *Filesystem {
	return &Filesystem{
		Filesystem:       base,
		unsavedDocuments: unsavedDocumentFs,
	}
}
