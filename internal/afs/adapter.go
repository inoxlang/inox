package afs

import (
	"io/fs"

	"github.com/go-git/go-billy/v5"
)

// FsPkgAdapter adapts a Filesystem to a fs.FS (stdlib).
type FsPkgAdapter struct {
	fs Filesystem
}

func MakeStdlibFsAdapter(fls Filesystem) fs.FS {
	return &FsPkgAdapter{
		fs: fls,
	}
}

func (a *FsPkgAdapter) Open(name string) (fs.File, error) {
	billyFile, err := a.fs.Open(name)
	if err != nil {
		return nil, err
	}

	return &FSPkgFileAdapter{
		openName: name,
		f:        billyFile,
		fs:       a.fs,
	}, nil
}

type FSPkgFileAdapter struct {
	openName string
	f        billy.File
	fs       billy.Filesystem
}

func (a FSPkgFileAdapter) Stat() (fs.FileInfo, error) {
	capable, ok := a.f.(StatCapable)
	if ok {
		return capable.Stat()
	}
	//Ok ?
	return a.fs.Stat(a.f.Name())
}

func (a FSPkgFileAdapter) Read(p []byte) (int, error) {
	return a.f.Read(p)
}
func (a FSPkgFileAdapter) Close() error {
	return a.f.Close()
}
