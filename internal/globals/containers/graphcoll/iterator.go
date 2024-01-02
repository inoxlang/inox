package graphcoll

import (
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/containers/common"
)

func (g *Graph) Iterator(ctx *core.Context, config core.IteratorConfiguration) core.Iterator {
	nodeIds := g.graph.NodeIds()
	i := -1

	return config.CreateIterator(&common.CollectionIterator{
		HasNext_: func(ci *common.CollectionIterator, ctx *core.Context) bool {
			if i < len(nodeIds)-1 {
				return true
			}
			nodeIds = nil
			return false
		},
		Next_: func(ci *common.CollectionIterator, ctx *core.Context) bool {
			if i >= len(nodeIds)-1 {
				nodeIds = nil
				return false
			}
			i++
			return true
		},
		Key_: func(ci *common.CollectionIterator, ctx *core.Context) core.Value {
			return core.Int(i)
		},
		Value_: func(ci *common.CollectionIterator, ctx *core.Context) core.Value {
			node := &GraphNode{id: nodeIds[i], graph: g}
			return node
		},
	})
}
