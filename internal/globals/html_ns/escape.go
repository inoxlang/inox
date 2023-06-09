package html_ns

import (
	"html"

	core "github.com/inoxlang/inox/internal/core"
)

func EscapeString(ctx *core.Context, s core.StringLike) core.Str {
	return core.Str(html.EscapeString(s.GetOrBuildString()))
}
