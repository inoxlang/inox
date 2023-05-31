package http_ns

import (
	core "github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/utils"
)

func PercentEncode(ctx *core.Context, s core.StringLike) core.Str {
	str := s.GetOrBuildString()
	return core.Str(utils.PercentEncode(str))
}

func PercentDecode(ctx *core.Context, s core.StringLike) (core.Str, core.Error) {
	str := s.GetOrBuildString()
	decoded, err := utils.PercentDecode(str, true)
	return core.Str(decoded), core.NewError(err, core.Nil)
}
