package strmanip_ns

import (
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/symbolic"
)

const (
	NAMESPACE_NAME = "strmanip"
)

func init() {
	core.RegisterSymbolicGoFunctions([]any{
		Titlecase, func(str symbolic.StringLike) symbolic.StringLike {
			return symbolic.ANY_STR_LIKE
		},
		Lowercase, func(str symbolic.StringLike) symbolic.StringLike {
			return symbolic.ANY_STR_LIKE
		},
		TrimSpace, func(str symbolic.StringLike) symbolic.StringLike {
			return symbolic.ANY_STR_LIKE
		},
	})
}

func NewStrManipNnamespace() *core.Namespace {
	return core.NewNamespace(NAMESPACE_NAME, map[string]core.Value{
		"title_case": core.WrapGoFunction(Titlecase),
		"lowercase":  core.WrapGoFunction(Lowercase),
		"trim_space": core.WrapGoFunction(TrimSpace),
	})
}
