package containers

import (
	"github.com/inoxlang/inox/internal/core"
)

func (pattern *SetPattern) Clone(clones map[uintptr]map[int]core.Value, depth int) (core.Value, error) {
	if depth > core.MAX_CLONING_DEPTH {
		return nil, core.ErrMaximumCloningDepthReached
	}

	return pattern, nil
}
