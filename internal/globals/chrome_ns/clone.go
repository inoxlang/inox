package chrome_ns

import core "github.com/inoxlang/inox/internal/core"

func (h *Handle) Clone(clones map[uintptr]map[int]core.Value) (core.Value, error) {
	return nil, core.ErrNotClonable
}
