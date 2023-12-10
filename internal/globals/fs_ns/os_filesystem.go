package fs_ns

import (
	"errors"

	"github.com/inoxlang/inox/internal/afs"
)

var (
	ErrOsFilesystemNotAvailable = errors.New("os filesystem not available")
	_                           = afs.OsFS((*OsFilesystem)(nil))
)
