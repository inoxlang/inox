package log_ns

import (
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/help"
	"github.com/inoxlang/inox/internal/inoxconsts"
	"github.com/rs/zerolog"
)

func init() {
	core.RegisterSymbolicGoFunctions([]any{
		_add, func(ctx *symbolic.Context, r *symbolic.Record) {
			ctx.SetSymbolicGoFunctionParameters(&SYMBOLIC_LOG_ADD_ARGS, SYMBOLIC_LOG_ADD_PARAM_NAMES)

			var hasMessageField bool
			var hasElements bool

			r.ForEachEntry(func(k string, v symbolic.Value) error {
				if k == zerolog.MessageFieldName {
					hasMessageField = true
				}

				if k == inoxconsts.IMPLICIT_PROP_NAME {
					hasElements = true
				}

				return nil
			})

			if hasMessageField && hasElements {
				ctx.AddSymbolicGoFunctionErrorf("the %q field should not be present if there are record elements", zerolog.MessageFieldName)
			}
		},
	})

	help.RegisterHelpValues(map[string]any{
		"log.add": _add,
	})
}
