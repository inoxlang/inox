//go:build unix

package internal

import (
	afs "github.com/inoxlang/inox/internal/afs"

	_fs "github.com/inoxlang/inox/internal/globals/fs"
)

// Filesystem is an implementation of billy.Filesystem that stores all files in a memory filesystem
type Filesystem struct {
	afs.Filesystem
	documents afs.Filesystem
}

func newFilesystem() *Filesystem {
	return &Filesystem{
		Filesystem: _fs.GetOsFilesystem(),
		documents:  _fs.NewMemFilesystem(),
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
