//go:build unix

package internal

import (
	afs "github.com/inoxlang/inox/internal/afs"

	"github.com/inoxlang/inox/internal/globals/fs_ns"
)

// Filesystem is an implementation of billy.Filesystem that stores all files in a memory filesystem
type Filesystem struct {
	afs.Filesystem
	documents afs.Filesystem
}

func NewDefaultFilesystem() *Filesystem {
	return &Filesystem{
		Filesystem: fs_ns.GetOsFilesystem(),
		documents:  fs_ns.NewMemFilesystem(),
	}
}

func NewFilesystem(base afs.Filesystem, editedDocumentFs afs.Filesystem) *Filesystem {
	if editedDocumentFs == nil {
		editedDocumentFs = base
	}

	return &Filesystem{
		Filesystem: base,
		documents:  editedDocumentFs,
	}
}

func (fs *Filesystem) Open(filename string) (afs.File, error) {
	f, err := fs.documents.Open(filename)
	if err != nil {
		return fs.Filesystem.Open(filename)
	}
	return f, nil
}

func (fs *Filesystem) docsFS() afs.Filesystem {
	return fs.documents
}
