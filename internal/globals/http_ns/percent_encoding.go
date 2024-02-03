package http_ns

import (
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/utils"
)

func PercentEncode(ctx *core.Context, s core.StringLike) core.String {
	str := s.GetOrBuildString()
	return core.String(utils.PercentEncode(str))
}

func PercentDecode(ctx *core.Context, s core.StringLike) (core.String, error) {
	str := s.GetOrBuildString()
	decoded, err := utils.PercentDecode(str, true)
	if err != nil {
		return "", core.NewError(err, core.Nil)
	}
	return core.String(decoded), nil
}
