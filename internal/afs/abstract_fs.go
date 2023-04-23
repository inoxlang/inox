package afs

import "github.com/go-git/go-billy/v5"

type Filesystem interface {
	billy.Filesystem
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
