package strmanip_ns

import (
	"strings"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/third_party_stable/titlecase"
)

func Titlecase(s core.StringLike) core.String {
	str := s.GetOrBuildString()
	return core.String(titlecase.Title(str))
}

func Lowercase(s core.StringLike) core.String {
	str := s.GetOrBuildString()
	return core.String(strings.ToLower(str))
}

func TrimSpace(s core.StringLike) core.String {
	str := s.GetOrBuildString()
	return core.String(strings.TrimSpace(str))
}
