package memds

import (
	"gonum.org/v1/gonum/graph/topo"
)

func (g *DirectedGraph[NodeData, EdgeData, InternalData]) HasCycle() bool {

	adapter := &simpleDirectedGraphAdapter[NodeData, EdgeData, InternalData]{
		graph: g,
	}

	cycles := topo.DirectedCyclesIn(adapter)
	return len(cycles) > 0
}
