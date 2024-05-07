package html_ns

import (
	"html"

	"github.com/inoxlang/inox/internal/core"
)

func EscapeString(ctx *core.Context, s core.StringLike) core.String {
	return core.String(html.EscapeString(s.GetOrBuildString()))
}

func UnescapeString(ctx *core.Context, s core.StringLike) core.String {
	return core.String(html.UnescapeString(s.GetOrBuildString()))
}
