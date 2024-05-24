package memds

import (
	"gonum.org/v1/gonum/graph/topo"
)

func (g *DirectedGraph[NodeData, EdgeData, InternalData]) HasCycle() bool {
	if g.lock != nil {
		g.lock.Lock()
		defer g.lock.Unlock()
	}

	adapter := &simpleDirectedGraphAdapter[NodeData, EdgeData, InternalData]{
		graph:         g,
		isGraphLocked: true,
	}

	cycles := topo.DirectedCyclesIn(adapter)
	return len(cycles) > 0
}

func (g *DirectedGraph[NodeData, EdgeData, InternalData]) hasCycleNoLock() bool {

	adapter := &simpleDirectedGraphAdapter[NodeData, EdgeData, InternalData]{
		graph:         g,
		isGraphLocked: true,
	}

	cycles := topo.DirectedCyclesIn(adapter)
	return len(cycles) > 0
}
