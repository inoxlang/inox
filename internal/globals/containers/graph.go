package internal

import (
	"errors"

	core "github.com/inox-project/inox/internal/core"
	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/simple"
)

const (
	nodeStackShrinkDivider       = 2
	minNodeShrinkableStackLength = 10 * nodeStackShrinkDivider
)

var (
	ErrEdgeListShouldHaveEvenLength = errors.New(`flat edge list should have an even length: [0, 1, 2, 3]`)
	ErrNodeAlreadyInGraph           = errors.New("node is already in the graph")
	ErrNodeNotInGraph               = errors.New("node is not in the graph")
)

func NewGraph(ctx *core.Context, nodes *core.List, edges *core.List) *Graph {

	g := &Graph{
		graph:  simple.NewDirectedGraph(),
		values: map[int64]core.Value{},
		roots:  make(map[int64]bool),
	}

	_nodes := make([]graph.Node, nodes.Len())

	nodeCount := nodes.Len()
	for i := 0; i < nodeCount; i++ {
		node := nodes.At(ctx, i)
		actualNode, _ := g.graph.NodeWithID(int64(i))
		g._insertNode(actualNode, node)
		_nodes[i] = actualNode
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
		g._connect(_nodes[from], _nodes[to])
	}

	return g
}

type Graph struct {
	core.NoReprMixin
	graph  *simple.DirectedGraph
	values map[int64]core.Value
	roots  map[int64]bool
}

func (g *Graph) InsertNode(ctx *core.Context, v core.Value) GraphNode {
	node := g.graph.NewNode()
	g._insertNode(node, v)
	return GraphNode{node_: node, graph: g}
}

func (g *Graph) _insertNode(node graph.Node, value core.Value) {
	id := node.ID()
	if g.graph.Node(id) != nil {
		panic(ErrNodeAlreadyInGraph)
	}
	g.roots[id] = true
	g.values[id] = value
	g.graph.AddNode(node)
}

func (g *Graph) RemoveNode(ctx *core.Context, node GraphNode) {
	if g.roots[node.node_.ID()] {
		it := g.graph.From(node.node_.ID())
		//we set as roots all children of the removed node that have no inbound edges.
		for it.Next() {
			to := it.Node()
			if g.graph.To(to.ID()).Len() == 0 {
				g.roots[to.ID()] = true
			}
		}
	}
	if g.graph.Node(node.node_.ID()) == nil {
		panic(ErrNodeAlreadyInGraph)
	}
	g.graph.RemoveNode(node.node_.ID())
}

func (g *Graph) Connect(ctx *core.Context, from, to GraphNode) {
	g._connect(from.node_, to.node_)
}

func (g *Graph) _connect(from, to graph.Node) {
	if g.roots[to.ID()] {
		delete(g.roots, to.ID())
		if g.graph.To(from.ID()).Len() == 0 {
			g.roots[from.ID()] = true
		}
	}
	edge := g.graph.NewEdge(from, to)
	g.graph.SetEdge(edge)
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
	length := g.graph.Nodes().Len()
	if length == 0 {
		return newEmptyGraphWalker(), nil
	}

	visited := make(map[int64]bool, length)
	stack := make([]int64, len(g.roots))
	current := int64(-1)
	firstChildStackIndex := -1 //used for pruning

	if len(g.roots) == 0 { //no roots
		//we start with a random node
		for nodeId := range g.values {
			stack = append(stack, nodeId)
			break
		}
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

			//loop through all chidren
			to := g.graph.From(e)

			if to.Len() == 0 {
				firstChildStackIndex = -1
			} else {
				firstChildStackIndex = ind
			}

			for to.Next() {
				child := to.Node()
				childId := child.ID()
				if visited[childId] {
					continue
				}

				visited[childId] = true
				stack = append(stack, childId)
			}

			//if the number of nodes is too small compared to the capacity of the stack we shrink the stack
			if len(stack) >= minNodeShrinkableStackLength && len(stack) <= cap(stack)/nodeStackShrinkDivider {
				newStack := make([]int64, len(stack))
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
			node, _ := g.graph.NodeWithID(current)
			return GraphNode{node_: node, graph: g}
		},
	}, nil
}

type GraphNode struct {
	core.NoReprMixin
	node_ graph.Node
	graph *Graph
}

func (n GraphNode) GetGoMethod(name string) (*core.GoFunction, bool) {
	return nil, false
}

func (n GraphNode) Prop(ctx *core.Context, name string) core.Value {
	switch name {
	case "data":
		return n.graph.values[n.node_.ID()]
	case "children", "parents":
		var it graph.Nodes
		if name == "children" {
			it = n.graph.graph.From(n.node_.ID())
		} else {
			it = n.graph.graph.To(n.node_.ID())
		}

		var next bool
		i := -1

		return &CollectionIterator{
			hasNext: func(ci *CollectionIterator, ctx *core.Context) bool {
				if !next {
					if !it.Next() {
						return false
					}
					next = true
				}
				return true
			},
			next: func(ci *CollectionIterator, ctx *core.Context) bool {
				next = false
				i++
				return true
			},
			key: func(ci *CollectionIterator, ctx *core.Context) core.Value {
				return core.Int(i)
			},
			value: func(ci *CollectionIterator, ctx *core.Context) core.Value {
				node := GraphNode{node_: it.Node(), graph: n.graph}
				return node
			},
		}
	}
	method, ok := n.GetGoMethod(name)
	if !ok {
		panic(core.FormatErrPropertyDoesNotExist(name, n))
	}
	return method
}

func (GraphNode) SetProp(ctx *core.Context, name string, value core.Value) error {
	return core.ErrCannotSetProp
}

func (GraphNode) PropertyNames(ctx *core.Context) []string {
	return []string{"data"}
}
