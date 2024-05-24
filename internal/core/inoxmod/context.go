package inoxmod

import (
	"context"

	"github.com/inoxlang/inox/internal/core/permbase"
)

type Context interface {
	context.Context
	CheckHasPermission(perm permbase.Permission) error
	CancelGracefully()
}
