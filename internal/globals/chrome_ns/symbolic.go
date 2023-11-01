package chrome_ns

import (
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/symbolic"
	chrome_symbolic "github.com/inoxlang/inox/internal/globals/chrome_ns/symbolic"
)

func init() {
}

func (h *Handle) ToSymbolicValue(ctx *core.Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return &chrome_symbolic.Handle{}, nil
}
