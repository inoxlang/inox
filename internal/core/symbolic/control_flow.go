package symbolic

import "github.com/inoxlang/inox/internal/memds"

// ControlFlowGraph is a thread-unsafe control flow graph of an Inox module.
type ControlFlowGraph struct {
	*memds.DirectedGraph[*ControlFlowNode, struct{}, struct{}]
}

func NewControlFlowGraph() *ControlFlowGraph {
	return &ControlFlowGraph{
		DirectedGraph: memds.NewDirectedGraph[*ControlFlowNode, struct{}](memds.ThreadUnsafe),
	}
}

type ControlFlowNode struct {
}
