package afs

import (
	"io/fs"

	"github.com/go-git/go-billy/v5"
)

const (
	DEFAULT_CREATE_FPERM fs.FileMode = 0600
)

type Filesystem interface {
	billy.Filesystem

	// Create creates the named file with mode 0600 (before umask), truncating
	// it if it already exists. If successful, methods on the returned File can
	// be used for I/O; the associated file descriptor has mode O_RDWR.
	Create(filename string) (File, error)

	Absolute(path string) (string, error)
}

type File = billy.File

type absoluteCapableFilesystem struct {
	billy.Filesystem
	absolute func(path string) (string, error)
}

func AddAbsoluteFeature(fls billy.Filesystem, absolute func(path string) (string, error)) Filesystem {
	return &absoluteCapableFilesystem{
		Filesystem: fls,
		absolute:   absolute,
	}
}

func (fls *absoluteCapableFilesystem) Absolute(path string) (string, error) {
	return fls.absolute(path)
}
