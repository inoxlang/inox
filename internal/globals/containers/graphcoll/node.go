package graphcoll

import (
	"sync/atomic"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/globals/containers/common"
	"github.com/inoxlang/inox/internal/memds"

	coll_symbolic "github.com/inoxlang/inox/internal/globals/containers/symbolic"
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

func (n *GraphNode) GetGoMethod(name string) (*core.GoFunction, bool) {
	return nil, false
}

func (n *GraphNode) Prop(ctx *core.Context, name string) core.Value {
	if n.removed.Load() {
		panic(ErrNodeNotInGraph)
	}
	switch name {
	case "data":
		data, ok := n.graph.graph.NodeData(n.id)
		if !ok {
			panic(ErrNodeNotInGraph)
		}
		return data
	case "children", "parents":
		var nodes []memds.GraphNode[core.Value]
		if name == "children" {
			nodes = n.graph.graph.DestinationNodes(n.id)
		} else {
			nodes = n.graph.graph.SourceNodes(n.id)
		}

		i := -1

		return &common.CollectionIterator{
			HasNext_: func(ci *common.CollectionIterator, ctx *core.Context) bool {
				if i < len(nodes)-1 {
					return true
				}
				nodes = nil
				return false
			},
			Next_: func(ci *common.CollectionIterator, ctx *core.Context) bool {
				if i >= len(nodes)-1 {
					nodes = nil
					return false
				}
				i++
				return true
			},
			Key_: func(ci *common.CollectionIterator, ctx *core.Context) core.Value {
				return core.Int(i)
			},
			Value_: func(ci *common.CollectionIterator, ctx *core.Context) core.Value {
				return &GraphNode{id: nodes[i].Id, graph: n.graph}
			},
		}
	}
	method, ok := n.GetGoMethod(name)
	if !ok {
		panic(core.FormatErrPropertyDoesNotExist(name, n))
	}
	return method
}

func (*GraphNode) SetProp(ctx *core.Context, name string, value core.Value) error {
	return core.ErrCannotSetProp
}

func (*GraphNode) PropertyNames(ctx *core.Context) []string {
	return []string{"data"}
}

func (n *GraphNode) IsMutable() bool {
	return true
}

func (n *GraphNode) Equal(ctx *core.Context, other core.Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherNode, ok := other.(*GraphNode)
	return ok && n.id == otherNode.id && n.removed.Load() != otherNode.removed.Load()
}

func (n *GraphNode) ToSymbolicValue(ctx *core.Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return &coll_symbolic.GraphNode{}, nil
}
