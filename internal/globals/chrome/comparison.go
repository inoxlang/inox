package internal

import core "github.com/inoxlang/inox/internal/core"

func (h *Handle) Equal(ctc *core.Context, other core.Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherHandle, ok := other.(*Handle)
	return ok && h == otherHandle
}
