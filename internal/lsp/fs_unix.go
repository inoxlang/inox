//go:build unix

package internal

import (
	afs "github.com/inoxlang/inox/internal/afs"

	"github.com/inoxlang/inox/internal/globals/fs_ns"
)

func NewDefaultFilesystem() *Filesystem {
	return &Filesystem{
		Filesystem:       fs_ns.GetOsFilesystem(),
		unsavedDocuments: fs_ns.NewMemFilesystem(DEFAULT_MAX_IN_MEM_FS_STORAGE_SIZE),
	}
}

func (fs *Filesystem) Open(filename string) (afs.File, error) {
	if fs.unsavedDocuments == nil {
		return fs.Filesystem.Open(filename)
	}

	f, err := fs.unsavedDocuments.Open(filename)
	if err != nil {
		return fs.Filesystem.Open(filename)
	}
	return f, nil
}

func (fs *Filesystem) docsFS() afs.Filesystem {
	if fs.unsavedDocuments == nil {
		return fs
	}
	return fs.unsavedDocuments
}

func (fs *Filesystem) Save() {

}
