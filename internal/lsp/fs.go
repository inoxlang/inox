package internal

import (
	afs "github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-billy/v5/osfs"
)

// Filesystem is an implementation of billy.Filesystem that stores the edited document files in a memory filesystem
type Filesystem struct {
	afs.Filesystem
	documents afs.Filesystem
}

func NewFilesystem() *Filesystem {
	return &Filesystem{
		Filesystem: osfs.New("/"),
		documents:  memfs.New(),
	}
}

func (fs *Filesystem) Open(filename string) (afs.File, error) {
	f, err := fs.documents.Open(filename)
	if err != nil {
		return fs.Filesystem.Open(filename)
	}
	return f, nil
}
