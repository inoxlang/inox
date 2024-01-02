package graphcoll

import (
	"errors"
	"fmt"
	"sync"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/memds"
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
