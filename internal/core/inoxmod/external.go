package inoxmod

import (
	"github.com/inoxlang/inox/internal/core/permbase"
	"github.com/inoxlang/inox/internal/parse"
)

var (
	CreatePath               func(absolutePath string) ResourceName
	CreateURL                func(url string) ResourceName
	EvalResourceNameLiteral  func(parse.SimpleValueLiteral) (ResourceName, error)
	CreateReadFilePermission func(absolutePath string) permbase.Permission
	CreateHttpReadPermission func(url string) permbase.Permission
	CreateBoundChildCtx      func(ctx Context) Context
)
