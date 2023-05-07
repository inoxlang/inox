package internal

import (
	core "github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/titlecase"
)

func Titlecase(s core.StringLike) core.Str {
	str := s.GetOrBuildString()
	return core.Str(titlecase.Title(str))
}
