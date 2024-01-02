package graphcoll

import (
	"sync/atomic"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/memds"
)

var (
	_ = core.Value((*GraphNode)(nil))
	_ = core.IProps((*GraphNode)(nil))
)

type GraphNode struct {
	id      memds.NodeId
	graph   *Graph
	removed atomic.Bool
}
