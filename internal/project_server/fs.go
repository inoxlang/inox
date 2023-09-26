package project_server

import (
	"errors"
	"os"

	"github.com/go-git/go-billy/v5"
	afs "github.com/inoxlang/inox/internal/afs"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
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

// OpenFile opens the unsaved document if flag is os.O_RDONLY, otherwise the persisted file is open.
func (fs *Filesystem) OpenFile(filename string, flag int, perm os.FileMode) (billy.File, error) {
	if fs_ns.IsReadOnly(flag) {
		f, err := fs.unsavedDocuments.OpenFile(filename, flag, 0)
		if os.IsNotExist(err) {
			return fs.Filesystem.OpenFile(filename, flag, 0)
		}
		return f, nil
	}
	return fs.Filesystem.OpenFile(filename, flag, perm)
}

func (fs *Filesystem) unsavedDocumentsFS() afs.Filesystem {
	return fs.unsavedDocuments
}
