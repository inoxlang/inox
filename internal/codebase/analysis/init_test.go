package analysis

import (
	"testing"

	"github.com/inoxlang/inox/internal/core"
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

		core.SetNewDefaultContext(func(config core.DefaultContextConfig) (*core.Context, error) {
			return core.NewContext(core.ContextConfig{}), nil
		})
		core.SetNewDefaultGlobalStateFn(func(ctx *core.Context, conf core.DefaultGlobalStateConfig) (*core.GlobalState, error) {
			return core.NewGlobalState(ctx), nil
		})
	}
}
