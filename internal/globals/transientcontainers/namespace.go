package transientcontainers

import (
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/symbolic"
	transientsymb "github.com/inoxlang/inox/internal/globals/transientcontainers/symbolic"
	"github.com/inoxlang/inox/internal/globals/transientcontainers/transientqueue"

	"github.com/inoxlang/inox/internal/help"
)

var ()

func init() {
	core.RegisterSymbolicGoFunctions([]any{
		transientqueue.NewQueue, func(ctx *symbolic.Context, elements symbolic.Iterable) *transientsymb.TransientQueue {
			return &transientsymb.TransientQueue{}
		},
	})

	help.RegisterHelpValues(map[string]any{})
}

func NewTransientContainersNamespace() map[string]core.Value {
	return map[string]core.Value{
		"TransientQueue": core.ValOf(transientqueue.NewQueue),
	}
}
