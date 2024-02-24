//go:build js

package fs_ns

import (
	"github.com/go-git/go-billy/v5"
)

type OsFilesystem struct {
	billy.Filesystem
}

func GetOsFilesystem() *OsFilesystem {
	panic(ErrOsFilesystemNotAvailable)
}

// we override Rename because osfs.OS.Rename is not the same as os.Rename
func (fs *OsFilesystem) Rename(from, to string) error {
	panic(ErrOsFilesystemNotAvailable)
}

func (fs OsFilesystem) Chroot(path string) (billy.Filesystem, error) {
	panic(ErrOsFilesystemNotAvailable)
}

func (fs OsFilesystem) Root() string {
	panic(ErrOsFilesystemNotAvailable)
}

func (fs OsFilesystem) Absolute(path string) (string, error) {
	panic(ErrOsFilesystemNotAvailable)
}
