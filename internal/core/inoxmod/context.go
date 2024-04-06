package inoxmod

import (
	"context"

	"github.com/inoxlang/inox/internal/afs"
	"github.com/inoxlang/inox/internal/core/permbase"
)

type Context interface {
	context.Context
	GetFileSystem() afs.Filesystem
	CheckHasPermission(perm permbase.Permission) error
	CancelGracefully()
}
