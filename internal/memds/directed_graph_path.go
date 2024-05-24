package memds

import (
	"errors"

	"github.com/bits-and-blooms/bitset"
	utils "github.com/inoxlang/inox/internal/utils/common"
	"golang.org/x/exp/slices"
)

// LongestPathLen returns one of the longest path found in g and the length of the path, if g has a cycle or a circuit -1 is returned.
// If there is no cycle or circuit len(nodesInPath) is equal to pathLength + 1.
func (g *DirectedGraph[NodeData, EdgeData, InternalData]) LongestPath() (nodesInPath []NodeId, pathLength int) {
	if g.lock != nil {
		g.lock.RLock()
		defer g.lock.RUnlock()
	}

	if len(g.nodes) <= 1 {
		return nil, 0
	}

	if g.hasCycleNoLock() {
		return nil, -1
	}

	//the length of the longest path starting from each node.
	longestPathLengths := make([]int32, g.currId+1)
	visited := bitset.New(uint(g.currId + 1))

	for nodeId := range g.nodes {
		if !visited.Test(uint(nodeId)) {
			g.longestPathLenDFS(nodeId, longestPathLengths, visited)
		}
	}

	allZeroes := true
	for _, length := range longestPathLengths {
		if length != 0 {
			allZeroes = false
			break
		}
	}
	if allZeroes {
		return nil, 0
	}

	type item struct {
		nodeId            NodeId
		longestPathLength int32
	}

	items := utils.MapSliceIndexed(longestPathLengths, func(longestPathLength int32, index int) item {
		return item{
			nodeId:            NodeId(index),
			longestPathLength: longestPathLength,
		}
	})

	//sort the longest path lengths in ascending order,
	//we created the items slice to keep the association (length, node id).
	slices.SortFunc(items, func(a, b item) int {
		return int(a.longestPathLength - b.longestPathLength)
	})

	//the last item has the longest path: its the first node in the path.
	lastItem := items[len(items)-1]
	longestPathLength := lastItem.longestPathLength

	path := make([]NodeId, lastItem.longestPathLength+1)
	startNode := lastItem.nodeId
	path[0] = startNode
	i := 1
	itemIndex := len(items) - 2

	prevLen := lastItem.longestPathLength
	currentChildren := g.from[startNode]

	visited.ClearAll()
	availableIds := visited //recycle the bitset

	for _, id := range g.availableIds {
		availableIds.Set(uint(id))
	}

	//reconstruct the path by searching the following nodes in the path
	for i < len(path) && itemIndex >= 0 {
		item := items[itemIndex]
		length := item.longestPathLength

		//if the id does not correspond to a node in the graph, we ignore it
		if availableIds.Test(uint(item.nodeId)) {
			itemIndex--
			continue
		}

		//if true the next node in path is item.nodeId
		if _, ok := currentChildren[item.nodeId]; ok && length != prevLen {
			prevLen = length
			currentChildren = g.from[item.nodeId]
			path[i] = item.nodeId
			i++
			continue
		}
		itemIndex--
	}

	if i != len(path) {
		panic(errors.New("all nodes should have been found"))
	}

	return path, int(longestPathLength)
}

// LongestPathLen returns the length of the longest path found in g, if g has a cycle or a circuit -1 is returned.
func (g *DirectedGraph[NodeData, EdgeData, InternalData]) LongestPathLen() int {
	if g.lock != nil {
		g.lock.RLock()
		defer g.lock.RUnlock()
	}

	if g.hasCycleNoLock() {
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
