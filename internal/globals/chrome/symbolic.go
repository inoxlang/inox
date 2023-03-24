package internal

import (
	symbolic "github.com/inox-project/inox/internal/core/symbolic"
	chrome_symbolic "github.com/inox-project/inox/internal/globals/chrome/symbolic"
)

func init() {
}

func (h *Handle) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &chrome_symbolic.Handle{}, nil
}
