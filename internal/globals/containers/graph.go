package containers

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	core "github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/in_mem_ds"
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

	_ = core.Value((*GraphNode)(nil))
	_ = core.IProps((*GraphNode)(nil))
)

func NewGraph(ctx *core.Context, nodeData *core.List, edges *core.List) *Graph {

	g := &Graph{
		graph: in_mem_ds.NewDirectedGraph[core.Value, struct{}](in_mem_ds.ThreadSafe),
		roots: make(map[in_mem_ds.NodeId]bool),
	}

	nodeIds := make([]in_mem_ds.NodeId, nodeData.Len())

	nodeCount := nodeData.Len()
	for i := 0; i < nodeCount; i++ {
		nodeData := nodeData.At(ctx, i)
		id := g.graph.AddNode(nodeData)
		g.roots[id] = true
		nodeIds[i] = id
	}

	if edges.Len()%2 != 0 {
		panic(ErrMapEntryListShouldHaveEvenLength)
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
	graph     *in_mem_ds.DirectedGraph[core.Value, struct{}, struct{}]
	roots     map[in_mem_ds.NodeId]bool
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

func (g *Graph) _connect(fromId, toId in_mem_ds.NodeId) {
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

	visited := make(map[in_mem_ds.NodeId]bool, length)
	stack := make([]in_mem_ds.NodeId, len(g.roots))
	current := in_mem_ds.NodeId(-1)
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
				newStack := make([]in_mem_ds.NodeId, len(stack))
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

type GraphNode struct {
	id      in_mem_ds.NodeId
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
		var nodes []in_mem_ds.GraphNode[core.Value]
		if name == "children" {
			nodes = n.graph.graph.DestinationNodes(n.id)
		} else {
			nodes = n.graph.graph.SourceNodes(n.id)
		}

		i := -1

		return &CollectionIterator{
			hasNext: func(ci *CollectionIterator, ctx *core.Context) bool {
				if i < len(nodes)-1 {
					return true
				}
				nodes = nil
				return false
			},
			next: func(ci *CollectionIterator, ctx *core.Context) bool {
				if i >= len(nodes)-1 {
					nodes = nil
					return false
				}
				i++
				return true
			},
			key: func(ci *CollectionIterator, ctx *core.Context) core.Value {
				return core.Int(i)
			},
			value: func(ci *CollectionIterator, ctx *core.Context) core.Value {
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
