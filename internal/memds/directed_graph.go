package memds

import (
	"errors"
	"sync"

	"github.com/inoxlang/inox/internal/utils"
	"golang.org/x/exp/maps"
)

var (
	ErrSelfEdgeNotSupportedYet        = errors.New("self edge not supported yet")
	ErrNodeNotFound                   = errors.New("node not found")
	ErrUnsupportedSingleNodeRetrieval = errors.New("unsupported single node retrieval")
)

// DirectedGraph is a directed graph.
type DirectedGraph[NodeData, EdgeData any, InternalData any] struct {
	nodes map[NodeId]GraphNode[NodeData]

	//source node -> destination nodes
	from map[NodeId]map[NodeId]EdgeData

	//destination node -> source nodes
	to map[NodeId]map[NodeId]EdgeData

	currId       NodeId
	availableIds []NodeId
	edgeCount    int64

	lock *sync.RWMutex //if nil the graph is not thread safe

	//the following fields should be set after the call to NewDirectedGraph and before any access to the graph.
	//if the graph is thread safe all calls are done while the graph is still locked.
	//all the following functions should never mutate the graph.

	additionalData   InternalData
	beforeAddingNode func(g *DirectedGraph[NodeData, EdgeData, InternalData], data NodeData) error
	afterNodeAdded   func(g *DirectedGraph[NodeData, EdgeData, InternalData], node GraphNode[NodeData])
	afterNodeRemoved func(g *DirectedGraph[NodeData, EdgeData, InternalData], id NodeId, data NodeData)

	//if the node is not found ErrNodeNotFound should be returned.
	//if the type of retrieval is not supported ErrUnsupportedSingleNodeRetrieval should be returned.
	getNodeWith func(g *DirectedGraph[NodeData, EdgeData, InternalData], retrieval SingleNodeRetrievalType, arg any) (GraphNode[NodeData], error)
}

type SingleNodeRetrievalType int

const (
	WithData SingleNodeRetrievalType = iota + 1
)

// NewDirectedGraph returns a DirectedGraph with no special abilities.
func NewDirectedGraph[NodeData, EdgeData any](threadSafety ThreadSafety) *DirectedGraph[NodeData, EdgeData, struct{}] {
	graph := &DirectedGraph[NodeData, EdgeData, struct{}]{
		nodes:  make(map[NodeId]GraphNode[NodeData]),
		from:   make(map[NodeId]map[NodeId]EdgeData),
		to:     make(map[NodeId]map[NodeId]EdgeData),
		currId: -1,
	}

	if threadSafety == ThreadSafe {
		graph.lock = &sync.RWMutex{}
	}

	return graph
}

// NewDirectedGraph returns a DirectedGraph.
func newDirectedGraph[NodeData, EdgeData any, InternalData any](threadSafety ThreadSafety) *DirectedGraph[NodeData, EdgeData, InternalData] {
	graph := &DirectedGraph[NodeData, EdgeData, InternalData]{
		nodes:  make(map[NodeId]GraphNode[NodeData]),
		from:   make(map[NodeId]map[NodeId]EdgeData),
		to:     make(map[NodeId]map[NodeId]EdgeData),
		currId: -1,
	}

	if threadSafety == ThreadSafe {
		graph.lock = &sync.RWMutex{}
	}

	return graph
}

func (g *DirectedGraph[NodeData, EdgeData, InternalData]) NodeCount() int {
	if g.lock != nil {
		g.lock.RLock()
		defer g.lock.RUnlock()
	}

	return len(g.nodes)
}

func (g *DirectedGraph[NodeData, EdgeData, InternalData]) EdgeCount() int64 {
	if g.lock != nil {
		g.lock.RLock()
		defer g.lock.RUnlock()
	}
	return g.edgeCount
}

// RandomNode returns the id of a pseudo randomly picked node.
func (g *DirectedGraph[NodeData, EdgeData, InternalData]) RandomNode() (NodeId, bool) {
	if g.lock != nil {
		g.lock.RLock()
		defer g.lock.RUnlock()
	}

	for nodeId := range g.nodes {
		return nodeId, true
	}
	return 0, false
}

// NodeIds returns all the node ids in the graph.
func (g *DirectedGraph[NodeData, EdgeData, InternalData]) NodeIds() []NodeId {
	if g.lock != nil {
		g.lock.RLock()
		defer g.lock.RUnlock()
	}

	return maps.Keys(g.nodes)
}

// AddNode creates an node with the passed data and returns the new node's id.
// Node ids start at 0.
func (g *DirectedGraph[NodeData, EdgeData, InternalData]) AddNode(data NodeData) NodeId {
	if g.lock != nil {
		g.lock.Lock()
		defer g.lock.Unlock()
	}

	if g.beforeAddingNode != nil {
		err := g.beforeAddingNode(g, data)
		if err != nil {
			panic(err)
		}
	}

	var id NodeId
	if len(g.availableIds) == 0 {
		g.currId++
		id = g.currId
	} else {
		id = g.availableIds[0]
		//shift
		copy(g.availableIds, g.availableIds[1:])
		g.availableIds = g.availableIds[:len(g.availableIds)-1]
	}

	node := GraphNode[NodeData]{
		Id:   id,
		Data: data,
	}

	g.nodes[id] = node

	if g.afterNodeAdded != nil {
		g.afterNodeAdded(g, node)
	}

	return id
}

// Edge returns the edge from srcId to destId if such an edge exists.
// The destination node must be directly reachable from the source node.
func (g *DirectedGraph[NodeData, EdgeData, InternalData]) Edge(srcId, destId NodeId) (GraphEdge[EdgeData], bool) {
	if g.lock != nil {
		g.lock.RLock()
		defer g.lock.RUnlock()
	}

	data, ok := g.from[srcId][destId]
	if !ok {
		return GraphEdge[EdgeData]{}, false
	}
	return GraphEdge[EdgeData]{From: srcId, To: destId, Data: data}, true
}

// Edges returns all the edges in the graph.
func (g *DirectedGraph[NodeData, EdgeData, InternalData]) Edges() []GraphEdge[EdgeData] {
	if g.lock != nil {
		g.lock.RLock()
		defer g.lock.RUnlock()
	}

	var edges []GraphEdge[EdgeData]
	for _, src := range g.nodes {
		for dest, data := range g.from[src.Id] {
			edges = append(edges, GraphEdge[EdgeData]{From: src.Id, To: dest, Data: data})
		}
	}

	//TODO: order edges
	return edges
}

// From returns all nodes in g that can be reached directly from n.
func (g *DirectedGraph[NodeData, EdgeData, InternalData]) DestinationNodes(id NodeId) []GraphNode[NodeData] {
	if g.lock != nil {
		g.lock.RLock()
		defer g.lock.RUnlock()
	}

	destIds := g.from[id]
	if len(destIds) == 0 {
		return nil
	}

	var nodes []GraphNode[NodeData]

	for destId := range destIds {
		nodes = append(nodes, utils.MustGet(g.Node(destId)))
	}

	return nodes
}

// From returns the ids of nodes in g that can be reached directly from n.
func (g *DirectedGraph[NodeData, EdgeData, InternalData]) DestinationIds(id NodeId) []NodeId {
	if g.lock != nil {
		g.lock.RLock()
		defer g.lock.RUnlock()
	}

	destIds := g.from[id]
	if len(destIds) == 0 {
		return nil
	}

	return maps.Keys(destIds)
}

// To returns all nodes in g that can reach directly to n.
func (g *DirectedGraph[NodeData, EdgeData, InternalData]) SourceNodes(id NodeId) []GraphNode[NodeData] {
	if g.lock != nil {
		g.lock.RLock()
		defer g.lock.RUnlock()
	}

	srcIds := g.to[id]
	if len(srcIds) == 0 {
		return nil
	}

	var nodes []GraphNode[NodeData]

	for srcId := range srcIds {
		nodes = append(nodes, utils.MustGet(g.Node(srcId)))
	}

	return nodes
}

// CountToTo returns the number of nodes in g that can reach directly to n.
func (g *DirectedGraph[NodeData, EdgeData, InternalData]) CountSourceNodes(id NodeId) int {
	if g.lock != nil {
		g.lock.RLock()
		defer g.lock.RUnlock()
	}

	srcIds := g.to[id]
	return len(srcIds)
}

// HasEdgeBetween returns whether an edge exists between nodes x and y without
// considering direction.
func (g *DirectedGraph[NodeData, EdgeData, InternalData]) HasEdgeBetween(xid, yid NodeId) bool {
	if g.lock != nil {
		g.lock.RLock()
		defer g.lock.RUnlock()
	}

	if _, ok := g.from[xid][yid]; ok {
		return true
	}
	_, ok := g.from[yid][xid]
	return ok
}

// HasEdgeFromTo returns whether an edge exists in the graph from srcId to destId.
func (g *DirectedGraph[NodeData, EdgeData, InternalData]) HasEdgeFromTo(srcId, destId NodeId) bool {
	if g.lock != nil {
		g.lock.RLock()
		defer g.lock.RUnlock()
	}

	if _, ok := g.from[srcId][destId]; !ok {
		return false
	}
	return true
}

// Node returns the node with the given ID if it exists in the graph.
func (g *DirectedGraph[NodeData, EdgeData, InternalData]) Node(id NodeId) (GraphNode[NodeData], bool) {
	if g.lock != nil {
		g.lock.RLock()
		defer g.lock.RUnlock()
	}

	node, ok := g.nodes[id]
	return node, ok
}

// Node returns the data of the node with the given ID if it exists in the graph.
func (g *DirectedGraph[NodeData, EdgeData, InternalData]) NodeData(id NodeId) (_ NodeData, _ bool) {
	if g.lock != nil {
		g.lock.RLock()
		defer g.lock.RUnlock()
	}

	node, ok := g.nodes[id]
	if ok {
		return node.Data, true
	}
	return
}

// GetNode retrieves a node using the specified retrieval type or returns the error ErrNodeNotFound.
// If the retrieval type is not supported ErrUnsupportedSingleNodeRetrieval is returned.
func (g *DirectedGraph[NodeData, EdgeData, InternalData]) GetNode(retrievalType SingleNodeRetrievalType, data any) (node GraphNode[NodeData], finalErr error) {
	if g.lock != nil {
		g.lock.RLock()
		defer g.lock.RUnlock()
	}

	if g.getNodeWith == nil {
		finalErr = ErrUnsupportedSingleNodeRetrieval
		return
	}

	return g.getNodeWith(g, retrievalType, data)
}

// HasNode determines if a node existing using the specified retrieval type.
// If the retrieval type is not supported ErrUnsupportedSingleNodeRetrieval is returned.
func (g *DirectedGraph[NodeData, EdgeData, InternalData]) HasNode(retrievalType SingleNodeRetrievalType, data any) (found bool, finalErr error) {
	if g.lock != nil {
		g.lock.RLock()
		defer g.lock.RUnlock()
	}

	if g.getNodeWith == nil {
		finalErr = ErrUnsupportedSingleNodeRetrieval
		return
	}

	_, err := g.getNodeWith(g, retrievalType, data)
	if errors.Is(err, ErrNodeNotFound) {
		return
	}
	if err != nil {
		finalErr = err
		return
	}

	return true, nil
}

// NodeWithID returns a Node with the given ID if found.
func (g *DirectedGraph[NodeData, EdgeData, InternalData]) NodeWithID(id NodeId) (n GraphNode[NodeData], found bool) {
	if g.lock != nil {
		g.lock.RLock()
		defer g.lock.RUnlock()
	}

	n, ok := g.nodes[NodeId(id)]
	if ok {
		return n, false
	}
	return
}

// RemoveEdge removes the edge with the given end point IDs from the graph, leaving the terminal
// nodes. If the edge does not exist it is a no-op.
func (g *DirectedGraph[NodeData, EdgeData, InternalData]) RemoveEdge(srcId, destId NodeId) {
	if g.lock != nil {
		g.lock.Lock()
		defer g.lock.Unlock()
	}

	if _, ok := g.nodes[srcId]; !ok {
		return
	}
	if _, ok := g.nodes[destId]; !ok {
		return
	}

	_, ok := g.from[srcId]
	if ok {
		g.edgeCount--
	}

	delete(g.from[srcId], destId)
	delete(g.to[destId], srcId)
}

// RemoveNode removes the node with the given ID from the graph, as well as any edges attached
// to it. If the node is not in the graph it is a no-op.
func (g *DirectedGraph[NodeData, EdgeData, InternalData]) RemoveNode(id NodeId) {
	if g.lock != nil {
		g.lock.Lock()
		defer g.lock.Unlock()
	}

	node, ok := g.nodes[id]
	if !ok {
		return
	}
	delete(g.nodes, id)

	for from := range g.from[id] {
		delete(g.to[from], id)
	}
	delete(g.from, id)

	for to := range g.to[id] {
		delete(g.from[to], id)
	}
	delete(g.to, id)

	if g.afterNodeRemoved != nil {
		g.afterNodeRemoved(g, id, node.Data)
	}

	g.availableIds = append(g.availableIds, id)
}

// SetEdge adds e, an edge from one node to another. The nodes must exist.
// It will panic if the target node is the same as the source node.
func (g *DirectedGraph[NodeData, EdgeData, InternalData]) SetEdge(from, to NodeId, data EdgeData) {
	e := GraphEdge[EdgeData]{
		From: from,
		To:   to,
		Data: data,
	}

	if e.From == e.To {
		panic(ErrSelfEdgeNotSupportedYet)
	}

	_, ok := g.nodes[e.From]
	if !ok {
		panic(ErrSrcNodeNotExist)
	}

	_, ok = g.nodes[e.To]
	if !ok {
		panic(ErrDestNodeNotExist)
	}

	//add edge in mapping SOURCE -> DESTINATION
	if fromMap, ok := g.from[e.From]; ok {
		if _, ok := fromMap[e.To]; !ok {
			g.edgeCount++
		}

		fromMap[e.To] = e.Data
	} else {
		g.edgeCount++
		g.from[e.From] = map[NodeId]EdgeData{e.To: e.Data}
	}

	//add edge in mapping DESTINATION -> SOURCE
	if toMap, ok := g.to[e.To]; ok {
		toMap[e.From] = e.Data
	} else {
		g.to[e.To] = map[NodeId]EdgeData{e.From: e.Data}
	}
}
