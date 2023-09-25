//go:build unix

package project_server

import (
	"github.com/inoxlang/inox/internal/globals/fs_ns"
)

func NewDefaultFilesystem() *Filesystem {
	return NewFilesystem(
		fs_ns.GetOsFilesystem(),
		fs_ns.NewMemFilesystem(DEFAULT_MAX_IN_MEM_FS_STORAGE_SIZE),
	)
}
