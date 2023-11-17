package fs_ns

import (
	"github.com/inoxlang/inox/internal/afs"
	"github.com/inoxlang/inox/internal/core"
)

var (
	_ = []ClosableFilesystem{(*MemFilesystem)(nil), (*MetaFilesystem)(nil)}
)

type ClosableFilesystem interface {
	afs.Filesystem
	Close(ctx *core.Context) error
}
