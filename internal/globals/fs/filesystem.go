package internal

import (
	afs "github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/osfs"

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

func (fs OsFilesystem) Chroot(path string) (afs.Filesystem, error) {
	return nil, core.ErrNotImplemented
}

func (fs OsFilesystem) Root() string {
	panic(core.ErrNotImplemented)
}
