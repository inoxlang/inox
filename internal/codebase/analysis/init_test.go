package analysis_test

import (
	"github.com/inoxlang/inox/internal/css/tailwind"
	"github.com/inoxlang/inox/internal/htmx"
	"github.com/inoxlang/inox/internal/hyperscript/hsparse"
	"github.com/inoxlang/inox/internal/parse"

	_ "github.com/inoxlang/inox/internal/globals"
)

func init() {
	tailwind.InitSubset()
	htmx.Load()
	parse.RegisterParseHypercript(hsparse.ParseHyperScriptProgram)
}
