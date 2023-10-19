package in_mem_ds

import "github.com/bits-and-blooms/bitset"

// (WIP)
// HasCycleOrCircuit detects if there is a cycle or a circuit in g,
// time complexity is O(N^2), space complexity is O(N).
// HasCycleOrCircuit only uses two bitsets of size ~N.
func (g *DirectedGraph[NodeData, EdgeData]) HasCycleOrCircuit() bool {
	if g.lock != nil {
		g.lock.RLock()
		defer g.lock.RUnlock()
	}

	toVisit := bitset.New(uint(g.currId + 1))
	visited := bitset.New(uint(g.currId + 1))

	for nodeId := range g.nodes {
		toVisit.ClearAll()
		visited.ClearAll()
		visited.Set(uint(nodeId))

		var current = nodeId
		destNodes := g.from[current]
		for dest := range destNodes {
			if visited.Test(uint(dest)) {
				return true
			}
			toVisit.Set(uint(dest))
		}

		if len(destNodes) == 0 {
			//since toVisit has not changed we
			//can check the next node.
			continue
		}

		//visit all nodes starting from the lowest node id.
		for {
			node, ok := toVisit.NextSet(0)
			if !ok {
				break
			}
			toVisit.Clear(node)
			if visited.Test(node) {
				return true
			}
			visited.Set(node)
			nodeId := NodeId(node)

			for dest := range g.from[nodeId] {
				if visited.Test(uint(dest)) {
					return true
				}
				toVisit.Set(uint(dest))
			}
			continue
		}
	}

	return false
}
