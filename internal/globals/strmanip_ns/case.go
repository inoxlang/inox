package strmanip_ns

import (
	"strings"

	core "github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/titlecase"
)

func Titlecase(s core.StringLike) core.Str {
	str := s.GetOrBuildString()
	return core.Str(titlecase.Title(str))
}

func Lowercase(s core.StringLike) core.Str {
	str := s.GetOrBuildString()
	return core.Str(strings.ToLower(str))
}

func TrimSpace(s core.StringLike) core.Str {
	str := s.GetOrBuildString()
	return core.Str(strings.TrimSpace(str))
}
