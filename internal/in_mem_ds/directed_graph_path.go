package in_mem_ds

import "github.com/bits-and-blooms/bitset"

// LongestPathLen returns the length of the longest path found in g, if g has a cycle or a circuit -1 is returned.
func (g *DirectedGraph[NodeData, EdgeData, InternalData]) LongestPathLen() int {
	if g.lock != nil {
		g.lock.RLock()
		defer g.lock.RUnlock()
	}

	if g.HasCycleOrCircuit() {
		return -1
	}

	//the length of the longest path starting from each node.
	longestPathLengths := make([]int32, g.currId+1)
	visited := bitset.New(uint(g.currId + 1))

	for nodeId := range g.nodes {
		if !visited.Test(uint(nodeId)) {
			g.longestPathLenDFS(nodeId, longestPathLengths, visited)
		}
	}

	maxPath := int32(0)
	for _, path := range longestPathLengths {
		if path > maxPath {
			maxPath = path
		}
	}
	return int(maxPath)
}

// longestPathLenDFS performs a depth free traversal of the graph, nodes in visited are not visited.
func (g *DirectedGraph[NodeData, EdgeData, InternalData]) longestPathLenDFS(node NodeId, longestPathLengths []int32, visited *bitset.BitSet) {
	visited.Set(uint(node))

	for child := range g.from[node] {
		if !visited.Test(uint(child)) {
			g.longestPathLenDFS(child, longestPathLengths, visited)
		}
		longestPathLengths[node] = max(longestPathLengths[node], 1+longestPathLengths[child])
	}
}
