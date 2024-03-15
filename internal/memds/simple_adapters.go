package memds

import (
	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/iterator"
)

var (
	_ graph.Directed = (*simpleDirectedGraphAdapter[int, int, int])(nil)
	_ graph.Node     = (*simpleNodeAdapter)(nil)
	_ graph.Edge     = (*simpleEdgeAdapter)(nil)
)

type simpleDirectedGraphAdapter[NodeData, EdgeData, InternalData any] struct {
	graph *DirectedGraph[NodeData, EdgeData, InternalData]
}

func (g *simpleDirectedGraphAdapter[NodeData, EdgeData, InternalData]) Edge(uid int64, vid int64) graph.Edge {
	_, ok := g.graph.Edge(NodeId(uid), NodeId(vid))
	if !ok {
		return nil
	}
	edgeAdapter := simpleEdgeAdapter{
		from: NodeId(uid),
		to:   NodeId(vid),
	}
	return &edgeAdapter
}

func (g *simpleDirectedGraphAdapter[NodeData, EdgeData, InternalData]) From(id int64) graph.Nodes {
	nodes := g.graph.DestinationNodes(NodeId(id))
	nodeMap := map[int64]graph.Node{}

	for _, node := range nodes {
		nodeMap[int64(node.Id)] = &simpleNodeAdapter{id: node.Id}
	}

	return iterator.NewNodes(nodeMap)
}

func (g *simpleDirectedGraphAdapter[NodeData, EdgeData, InternalData]) To(id int64) graph.Nodes {
	nodes := g.graph.SourceNodes(NodeId(id))
	nodeMap := map[int64]graph.Node{}

	for _, node := range nodes {
		nodeMap[int64(node.Id)] = &simpleNodeAdapter{id: node.Id}
	}

	return iterator.NewNodes(nodeMap)
}

func (g *simpleDirectedGraphAdapter[NodeData, EdgeData, InternalData]) HasEdgeBetween(xid int64, yid int64) bool {
	return g.graph.HasEdgeBetween(NodeId(xid), NodeId(yid))
}

func (g *simpleDirectedGraphAdapter[NodeData, EdgeData, InternalData]) HasEdgeFromTo(uid int64, vid int64) bool {
	return g.graph.HasEdgeFromTo(NodeId(uid), NodeId(vid))
}

func (g *simpleDirectedGraphAdapter[NodeData, EdgeData, InternalData]) Node(id int64) graph.Node {
	node, ok := g.graph.Node(NodeId(id))
	if ok {
		return &simpleNodeAdapter{id: node.Id}
	}
	return nil
}

func (g *simpleDirectedGraphAdapter[NodeData, EdgeData, InternalData]) Nodes() graph.Nodes {
	nodeMap := map[int64]graph.Node{}

	g.graph.forEachNodeIdInternal(func(id NodeId) {
		nodeMap[int64(id)] = &simpleNodeAdapter{id: id}
	})

	return iterator.NewNodes(nodeMap)
}

// To implements graph.Directed.

type simpleNodeAdapter struct {
	id NodeId
}

func (s simpleNodeAdapter) ID() int64 {
	return int64(s.id)
}

type simpleEdgeAdapter struct {
	from, to NodeId
}

func (g *simpleEdgeAdapter) From() graph.Node {
	return &simpleNodeAdapter{g.from}
}

func (g *simpleEdgeAdapter) To() graph.Node {
	return &simpleNodeAdapter{g.to}
}

func (g *simpleEdgeAdapter) ReversedEdge() graph.Edge {
	return &simpleEdgeAdapter{
		from: g.to,
		to:   g.from,
	}
}
