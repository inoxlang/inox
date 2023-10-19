package in_mem_ds

import (
	"errors"
)

var (
	ErrNodeSameStringDataAlreadyInGraph = errors.New("a node with the same data is already in the graph")
)

// NewDirectedGraphUniqueString returns a directed graph that supports WithData, adding two nodes with the same data is forbidden.
func NewDirectedGraphUniqueString[NodeData ~string, EdgeData any](threadSafety ThreadSafety) *DirectedGraph[NodeData, EdgeData, map[NodeData]NodeId] {
	graph := newDirectedGraph[NodeData, EdgeData, map[NodeData]NodeId](threadSafety)

	type G = *DirectedGraph[NodeData, EdgeData, map[NodeData]NodeId]
	type N = GraphNode[NodeData]

	graph.beforeAddingNode = func(g G, data NodeData) error {
		_, alreadyPresent := g.additionalData[data]
		if alreadyPresent {
			return ErrNodeSameStringDataAlreadyInGraph
		}
		return nil
	}

	graph.afterNodeAdded = func(g G, node N) {
		g.additionalData[node.Data] = node.Id
	}

	graph.afterNodeRemoved = func(g G, id NodeId, data NodeData) {
		delete(g.additionalData, data)
	}

	graph.additionalData = map[NodeData]NodeId{}

	graph.getNodeWith = func(g G, retrieval SingleNodeRetrievalType, arg any) (_ N, finalErr error) {
		switch retrieval {
		case WithData:
			id, ok := g.additionalData[arg.(NodeData)]

			if !ok {
				finalErr = ErrNodeNotFound
				return
			}
			node, ok := g.nodes[id]
			if !ok {
				finalErr = ErrNodeNotFound
				return
			}

			return node, nil
		default:
			finalErr = ErrUnsupportedSingleNodeRetrieval
			return
		}
	}

	return graph
}
