package project_server

import (
	"errors"

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

// NewFilesystem creates a new Filesystem with a persistsed filesystem and a filesystem
// for storing the state of unsave documents. unsavedDocumentFs should be fast.
func NewFilesystem(base afs.Filesystem, unsavedDocumentFs afs.Filesystem) *Filesystem {
	if unsavedDocumentFs == nil {
		panic(errors.New("unsavedDocumentFs is nil"))
	}
	return &Filesystem{
		Filesystem:       base,
		unsavedDocuments: unsavedDocumentFs,
	}
}

func (fs *Filesystem) unsavedDocumentsFS() afs.Filesystem {
	return fs.unsavedDocuments
}
