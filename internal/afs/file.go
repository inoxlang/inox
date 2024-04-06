package afs

import (
	"errors"
	"os"

	"github.com/go-git/go-billy/v5"
)

var (
	ErrNotStatCapable = errors.New("not stat capable")
)

type StatCapable interface {
	billy.File
	Stat() (os.FileInfo, error)
}

type SyncCapable interface {
	billy.File
	Sync() error
}

// FileStat tries to directly use the given file to get file information,
// if it fails and fls is not nil then fls.Stat(f) is used.
func FileStat(f billy.File, fls billy.Basic) (os.FileInfo, error) {
	interf, ok := f.(interface{ Stat() (os.FileInfo, error) })
	if !ok {
		return fls.Stat(f.Name())
	}
	return interf.Stat()
}
