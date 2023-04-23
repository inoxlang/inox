package internal

import (
	"path/filepath"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/memfs"
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

func (fs OsFilesystem) Chroot(path string) (billy.Filesystem, error) {
	return nil, core.ErrNotImplemented
}

func (fs OsFilesystem) Root() string {
	panic(core.ErrNotImplemented)
}

func (fs OsFilesystem) Absolute(path string) (string, error) {
	return filepath.Abs(path)
}

type MemFilesystem struct {
	billy.Filesystem
}

func NewMemFilesystem() *MemFilesystem {
	return &MemFilesystem{
		Filesystem: memfs.New(),
	}
}

func (fs MemFilesystem) Chroot(path string) (billy.Filesystem, error) {
	return nil, core.ErrNotImplemented
}

func (fs MemFilesystem) Root() string {
	panic(core.ErrNotImplemented)
}

func (fs MemFilesystem) Absolute(path string) (string, error) {
	return "", core.ErrNotImplemented
}
