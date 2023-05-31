//go:build js

package internal

import (
	afs "github.com/inoxlang/inox/internal/afs"
	fs_ns "github.com/inoxlang/inox/internal/globals/fs_ns"
)

// Filesystem is a filesystem that stores all files in a memory filesystem.
type Filesystem struct {
	*fs_ns.MemFilesystem
}

func NewDefaultFilesystem() *Filesystem {
	return &Filesystem{
		fs_ns.NewMemFilesystem(),
	}
}

func (fs *Filesystem) docsFS() afs.Filesystem {
	return fs
}
