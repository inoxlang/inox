package internal

import (
	"os"
	"path/filepath"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/osfs"
	afs "github.com/inoxlang/inox/internal/afs"

	core "github.com/inoxlang/inox/internal/core"
)

var (
	osFs = &OsFilesystem{
		OS: *osfs.Default,
	}

	_ afs.Filesystem = osFs
)

type OsFilesystem struct {
	osfs.OS
}

func GetOsFilesystem() *OsFilesystem {
	return osFs
}

// we override Rename because osfs.OS.Rename is not the same as os.Rename
func (fs *OsFilesystem) Rename(from, to string) error {
	return os.Rename(from, to)
}

func (fs OsFilesystem) Chroot(path string) (billy.Filesystem, error) {
	return nil, core.ErrNotImplemented
}

func (fs OsFilesystem) Root() string {
	panic(core.ErrNotImplemented)
}

func (fs OsFilesystem) Absolute(path string) (string, error) {
	return filepath.Abs(path)
}
