package in_mem_ds

import (
	"errors"

	"github.com/inoxlang/inox/internal/utils"
	"golang.org/x/exp/constraints"
)

var (
	ErrFullGraph32 = errors.New("Graph32 is full")
)

// A Graph32 is a directed graph that performs no allocations and has a capacity of 32 nodes.
type Graph32[T constraints.Ordered] struct {
	size     int
	edges    [32]BitSet32
	nodeData [32]T
}

type Graph32Node[T any] struct {
	id   Bit32Index
	data T
}

func (n Graph32Node[T]) Id() NodeId {
	return NodeId(n.id)
}

func (g Graph32[T]) HasNodeOfId(id NodeId) bool {
	return id >= 0 && id < NodeId(g.size)
}

func (g Graph32[T]) NodeOfId(id NodeId) (node Graph32Node[T], found bool) {
	if !g.HasNodeOfId(id) {
		return
	}

	return Graph32Node[T]{
		id:   Bit32Index(id),
		data: g.nodeData[id],
	}, true
}

func (g *Graph32[T]) AddNode(data T) Graph32Node[T] {
	if int(g.size) >= len(g.edges) {
		panic(ErrFullGraph32)
	}

	id := Bit32Index(g.size)
	g.size++
	g.nodeData[id] = data
	g.edges[id] = 0
	return Graph32Node[T]{
		id:   id,
		data: data,
	}
}

func (g Graph32[T]) IdOfNode(v T) (NodeId, bool) {
	for id, e := range g.nodeData {
		if e == v {
			return NodeId(id), true
		}
	}
	return 0, false
}

func (g Graph32[T]) MustGetIdOfNode(v T) NodeId {
	return utils.MustGet(g.IdOfNode(v))
}

func (g *Graph32[T]) AddEdge(fromNodeId, toNodeId NodeId) {
	if fromNodeId < 0 || fromNodeId > g.maxAllowedId() || toNodeId < 0 || toNodeId > g.maxAllowedId() {
		panic(ErrInvalidOrOutOfBoundsNodeId)
	}

	g.edges[fromNodeId].Set(Bit32Index(toNodeId))
}

func (g Graph32[T]) HasEdgeFromTo(fromNodeId, toNodeId NodeId) bool {
	if !g.isValidNodeId(fromNodeId) || !g.isValidNodeId(toNodeId) {
		panic(ErrInvalidOrOutOfBoundsNodeId)
	}

	return g.edges[fromNodeId].IsSet(Bit32Index(toNodeId))
}

func (g Graph32[T]) isValidNodeId(nodeId NodeId) bool {
	return nodeId >= 0 && nodeId <= g.maxAllowedId()
}

// IteratorDirectlyReachableNodes returns an iterator that iterates over all nodes directly and indirectly reachable
// from the node of id fromNodeId. The iteration start at this node.
func (g Graph32[T]) IteratorFrom(fromNodeId NodeId) Graph32Iterator[T] {
	if !g.isValidNodeId(fromNodeId) {
		panic(ErrInvalidOrOutOfBoundsNodeId)
	}

	if !g.HasNodeOfId(fromNodeId) {
		visited := BitSet32(0)
		visited.SetAll()

		return Graph32Iterator[T]{
			visited: visited,
		}
	}

	nodes := g.edges[fromNodeId]

	it := Graph32Iterator[T]{
		copy:    g,
		visited: BitSet32(0),
	}

	nodes.ForEachSet(func(index Bit32Index) error {
		it.currentNodes.Set(index)
		return nil
	})

	return it
}

// IteratorDirectlyReachableNodes returns an iterator that iterates over the nodes directly reachable from the node of id fromNodeId.
func (g Graph32[T]) IteratorDirectlyReachableNodes(fromNodeId NodeId) Graph32Iterator[T] {
	if !g.isValidNodeId(fromNodeId) {
		panic(ErrInvalidOrOutOfBoundsNodeId)
	}

	if !g.HasNodeOfId(fromNodeId) {
		return Graph32Iterator[T]{
			finished: true,
		}
	}

	nodes := g.edges[fromNodeId]

	it := Graph32Iterator[T]{
		copy:    g,
		visited: BitSet32(0),
	}

	it.visited.SetAll()

	nodes.ForEachSet(func(index Bit32Index) error {
		it.visited.Unset(index)
		it.currentNodes.Set(index)
		return nil
	})

	return it
}

func (g *Graph32[T]) Size() int {
	return g.size
}

func (g *Graph32[T]) Capacity() int {
	return len(g.edges)
}

func (g *Graph32[T]) maxAllowedId() NodeId {
	return NodeId(g.size - 1)
}

type Graph32Iterator[T constraints.Ordered] struct {
	copy         Graph32[T]
	currentNodes BitSet32
	currentNode  Bit32Index
	visited      BitSet32
	finished     bool
}

func (it *Graph32Iterator[T]) Node() Graph32Node[T] {
	return utils.MustGet(it.copy.NodeOfId(NodeId(it.currentNode)))
}

func (it *Graph32Iterator[T]) Next() bool {
	ok := false

	it.currentNodes.ForEachSet(func(index Bit32Index) error {
		if !it.visited.IsSet(index) {
			it.visited.Set(index)
			ok = true
			it.currentNode = index
			edges := it.copy.edges[index]

			edges.ForEachSet(func(linkedNode Bit32Index) error {
				if !it.visited.IsSet(linkedNode) {
					it.currentNodes.Set(linkedNode)
				}
				return nil
			})
		}
		return nil
	})

	return ok
}
