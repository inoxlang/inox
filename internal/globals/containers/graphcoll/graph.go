package graphcoll

import (
	"errors"
	"fmt"
	"sync"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/memds"

	coll_symbolic "github.com/inoxlang/inox/internal/globals/containers/symbolic"
)

const (
	nodeStackShrinkDivider       = 2
	minNodeShrinkableStackLength = 10 * nodeStackShrinkDivider
)

var (
	ErrEdgeListShouldHaveEvenLength = errors.New(`flat edge list should have an even length: [0, 1, 2, 3]`)
	ErrNodeAlreadyInGraph           = errors.New("node is already in the graph")
	ErrNodeNotInGraph               = errors.New("node is not in the graph")

	_ = core.Value((*Graph)(nil))
	_ = core.IProps((*Graph)(nil))
)

func NewGraph(ctx *core.Context, nodeData *core.List, edges *core.List) *Graph {

	g := &Graph{
		graph: memds.NewDirectedGraph[core.Value, struct{}](memds.ThreadSafe),
		roots: make(map[memds.NodeId]bool),
	}

	nodeIds := make([]memds.NodeId, nodeData.Len())

	nodeCount := nodeData.Len()
	for i := 0; i < nodeCount; i++ {
		nodeData := nodeData.At(ctx, i)
		id := g.graph.AddNode(nodeData)
		g.roots[id] = true
		nodeIds[i] = id
	}

	if edges.Len()%2 != 0 {
		panic(ErrEdgeListShouldHaveEvenLength)
	}

	halfEdgeCount := edges.Len() / 2
	for i := 0; i < halfEdgeCount; i += 2 {
		fromId := edges.At(ctx, i)
		toId := edges.At(ctx, i+1)

		from := fromId.(core.Int)
		to := toId.(core.Int)
		g._connect(nodeIds[from], nodeIds[to])
	}

	return g
}

type Graph struct {
	graph     *memds.DirectedGraph[core.Value, struct{}, struct{}]
	roots     map[memds.NodeId]bool
	rootsLock sync.Mutex
}

func (g *Graph) InsertNode(ctx *core.Context, v core.Value) *GraphNode {
	id := g.graph.AddNode(v)

	g.rootsLock.Lock()
	g.roots[id] = true
	g.rootsLock.Unlock()

	return &GraphNode{id: id, graph: g}
}

func (g *Graph) RemoveNode(ctx *core.Context, node *GraphNode) {
	if !node.removed.CompareAndSwap(false, true) {
		return
	}

	if _, ok := g.graph.Node(node.id); !ok {
		panic(ErrNodeNotInGraph)
	}
	g.graph.RemoveNode(node.id)
	g.rootsLock.Lock()

	if g.roots[node.id] {
		g.rootsLock.Unlock()

		//we set as roots all children of the removed node that have no other inbound edges.
		for _, destinationId := range g.graph.DestinationIds(node.id) {
			if g.graph.CountSourceNodes(destinationId) == 0 {
				g.rootsLock.Lock()
				g.roots[destinationId] = true
				g.rootsLock.Unlock()
			}
		}
	} else {
		g.rootsLock.Unlock()
	}
}

func (g *Graph) Connect(ctx *core.Context, from, to *GraphNode) {
	if from.removed.Load() {
		panic(fmt.Errorf("source node: %w", ErrNodeNotInGraph))
	}
	if to.removed.Load() {
		panic(fmt.Errorf("source node: %w", ErrNodeNotInGraph))
	}

	g._connect(from.id, to.id)
}

func (g *Graph) _connect(fromId, toId memds.NodeId) {
	if g.roots[toId] {
		delete(g.roots, toId)

		//we set as root the source node if .
		if g.graph.CountSourceNodes(fromId) == 0 {
			g.roots[fromId] = true
		}
	}

	g.graph.SetEdge(fromId, toId, struct{}{})
}

func (f *Graph) GetGoMethod(name string) (*core.GoFunction, bool) {
	switch name {
	case "insert_node":
		return core.WrapGoMethod(f.InsertNode), true
	case "remove_node":
		return core.WrapGoMethod(f.RemoveNode), true
	case "connect":
		return core.WrapGoMethod(f.Connect), true
	}
	return nil, false
}

func (g *Graph) Prop(ctx *core.Context, name string) core.Value {
	method, ok := g.GetGoMethod(name)
	if !ok {
		panic(core.FormatErrPropertyDoesNotExist(name, g))
	}
	return method
}

func (*Graph) SetProp(ctx *core.Context, name string, value core.Value) error {
	return core.ErrCannotSetProp
}

func (*Graph) PropertyNames(ctx *core.Context) []string {
	return []string{"insert_node", "remove_node"}
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

func (g *Graph) IsMutable() bool {
	return true
}

func (g *Graph) Equal(ctx *core.Context, other core.Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherGraph, ok := other.(*Graph)
	return ok && g == otherGraph
}

func (g *Graph) ToSymbolicValue(ctx *core.Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return &coll_symbolic.Graph{}, nil
}
