package analysis

import (
	"testing"

	"github.com/inoxlang/inox/internal/css/tailwind"
	"github.com/inoxlang/inox/internal/htmx"
	"github.com/inoxlang/inox/internal/hyperscript/hsparse"
	"github.com/inoxlang/inox/internal/parse"
)

func init() {
	if testing.Testing() {
		tailwind.InitSubset()
		htmx.Load()
		parse.RegisterParseHypercript(hsparse.ParseHyperScript)
	}
}
