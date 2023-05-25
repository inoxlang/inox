//go:build js

package internal

import (
	afs "github.com/inoxlang/inox/internal/afs"
	_fs "github.com/inoxlang/inox/internal/globals/fs"
)

// Filesystem is an implementation of billy.Filesystem that stores the edited document files in a memory filesystem
type Filesystem struct {
	*_fs.MemFilesystem
}

func newFilesystem() *Filesystem {
	return &Filesystem{
		_fs.NewMemFilesystem(),
	}
}

func (fs *Filesystem) docsFS() afs.Filesystem {
	return fs
}
