package graphcoll

import (
	"bufio"
	"fmt"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/memds"
	"github.com/inoxlang/inox/internal/utils"
)

var (
	_ = []core.Walkable{(*Graph)(nil)}
)

type GraphWalker struct {
	hasNext func(*GraphWalker, *core.Context) bool
	next    func(*GraphWalker, *core.Context) bool
	prune   func(*GraphWalker, *core.Context)
	key     func(*GraphWalker, *core.Context) core.Value
	value   func(*GraphWalker, *core.Context) core.Value
}

func (g *Graph) Walker(*core.Context) (core.Walker, error) {
	length := g.graph.NodeCount()
	if length == 0 {
		return newEmptyGraphWalker(), nil
	}

	visited := make(map[memds.NodeId]bool, length)
	stack := make([]memds.NodeId, len(g.roots))
	current := memds.NodeId(-1)
	firstChildStackIndex := -1 //used for pruning

	if len(g.roots) == 0 { //no roots
		//we start with a random node
		nodeId, ok := g.graph.RandomNode()
		if !ok {
			return newEmptyGraphWalker(), nil
		}
		stack = append(stack, nodeId)
	} else {
		i := 0
		for rootId := range g.roots {
			stack[i] = rootId
			i++
		}
	}

	return &GraphWalker{
		hasNext: func(wk *GraphWalker, ctx *core.Context) bool {
			return len(stack) > 0
		},
		next: func(wk *GraphWalker, ctx *core.Context) bool {
			ind := len(stack) - 1
			e := stack[ind]
			stack = stack[:ind]

			current = e
			visited[e] = true

			//loop through all destination nodes ("children")
			destinationNodes := g.graph.DestinationNodes(e)

			if len(destinationNodes) == 0 {
				firstChildStackIndex = -1
			} else {
				firstChildStackIndex = ind
			}

			for _, destNode := range destinationNodes {
				destId := destNode.Id
				if visited[destId] {
					continue
				}

				visited[destId] = true
				stack = append(stack, destId)
			}

			//if the number of nodes is too small compared to the capacity of the stack we shrink the stack
			if len(stack) >= minNodeShrinkableStackLength && len(stack) <= cap(stack)/nodeStackShrinkDivider {
				newStack := make([]memds.NodeId, len(stack))
				copy(newStack, stack)
				stack = newStack
			}

			return true
		},
		prune: func(wk *GraphWalker, ctx *core.Context) {
			if firstChildStackIndex >= 0 {
				stack = stack[:firstChildStackIndex]
			}
		},
		key: func(wk *GraphWalker, ctx *core.Context) core.Value {
			return core.Int(current)
		},
		value: func(wk *GraphWalker, ctx *core.Context) core.Value {
			node, ok := g.graph.NodeWithID(current)
			if !ok {
				panic(ErrNodeNotInGraph)
			}
			return &GraphNode{id: node.Id, graph: g}
		},
	}, nil
}

func (wk *GraphWalker) HasNext(ctx *core.Context) bool {
	return wk.hasNext(wk, ctx)
}

func (wk *GraphWalker) Next(ctx *core.Context) bool {
	if !wk.HasNext(ctx) {
		return false
	}

	return wk.next(wk, ctx)
}

func (wk *GraphWalker) Prune(ctx *core.Context) {
	wk.prune(wk, ctx)
}

func (wk *GraphWalker) Key(ctx *core.Context) core.Value {
	return wk.key(wk, ctx)
}

func (wk *GraphWalker) Value(ctx *core.Context) core.Value {
	return wk.value(wk, ctx)
}

func (wk *GraphWalker) NodeMeta(*core.Context) core.WalkableNodeMeta {
	panic(core.ErrNotImplementedYet)
}

func (wk *GraphWalker) Equal(ctx *core.Context, other core.Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherWk, ok := other.(*GraphWalker)
	return ok && wk == otherWk
}

func (wk *GraphWalker) PrettyPrint(w *bufio.Writer, config *core.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(fmt.Fprintf(w, "%#v", wk))
}

func (it *GraphWalker) ToSymbolicValue(ctx *core.Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return nil, symbolic.ErrNoSymbolicValue
}

func (wk *GraphWalker) IsMutable() bool {
	return true
}

func newEmptyGraphWalker() *GraphWalker {
	return &GraphWalker{
		hasNext: func(gw *GraphWalker, ctx *core.Context) bool {
			return false
		},
		next: func(gw *GraphWalker, ctx *core.Context) bool {
			return false
		},
	}
}
