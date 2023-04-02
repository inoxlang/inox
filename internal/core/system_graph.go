package internal

import (
	"errors"
	"fmt"
	"reflect"
	"runtime"
	"sync"
	"unsafe"
)

const (
	// This mutation adds a single node + an optional edge
	SG_AddNode SpecificMutationKind = iota + 1

	// This mutation adds a single edge
	SG_AddEdge

	// This mutation adds a single event
	SG_AddEvent
)

const (
	DEFAULT_EDGE_TO_CHILD_TEXT         = "parent of"
	DEFAULT_EDGE_TO_WATCHED_CHILD_TEXT = "watching"
)

var (
	ErrValueAlreadyInSysGraph = errors.New("value already in a system graph")
	ErrValueNotInSysGraph     = errors.New("value is not part of system graph")
	ErrValueNotPointer        = errors.New("value is not a pointer")

	SYSTEM_GRAPH_PROPNAMES       = []string{"nodes", "events"}
	SYSTEM_GRAPH_EVENT_PROPNAMES = []string{"text", "value0_id"}
	SYSTEM_GRAP_EDGE_PROPNAMES   = []string{"to", "text"}
	SYSTEM_GRAPH_NODE_PROPNAMES  = []string{"name", "type_name", "value_id", "edges"}

	_ = []PotentiallySharable{(*SystemGraph)(nil), (*SystemGraphNodes)(nil)}
	_ = []IProps{(*SystemGraph)(nil), (*SystemGraphNode)(nil)}
	_ = []Iterable{(*SystemGraphNodes)(nil)}
)

// A SystemGraph represents relations & events between values.
type SystemGraph struct {
	nodes *SystemGraphNodes

	eventLogLock sync.Mutex
	eventLog     []SystemGraphEvent

	mutationCallbacks *MutationCallbacks
	isFrozen          bool         // SystemGraph should not supported unfreezing
	lastSnapshot      *SystemGraph // discarded when there is a mutation

	NoReprMixin
	NotClonableMixin
}

func NewSystemGraph() *SystemGraph {
	g := &SystemGraph{}

	g.nodes = &SystemGraphNodes{
		graph:     g,
		ptrToNode: make(map[uintptr]*SystemGraphNode),
	}

	return g
}

type SystemGraphEdge struct {
	text string
	to   uintptr
	kind SystemGraphEdgeKind

	NoReprMixin
	NotClonableMixin
}

func (e SystemGraphEdge) Prop(ctx *Context, name string) Value {
	switch name {
	case "to":
		return Int(e.to)
	case "text":
		return Str(e.text)
	}
	panic(FormatErrPropertyDoesNotExist(name, e))
}

func (SystemGraphEdge) SetProp(ctx *Context, name string, value Value) error {
	return ErrCannotSetProp
}

func (SystemGraphEdge) PropertyNames(ctx *Context) []string {
	return SYSTEM_GRAP_EDGE_PROPNAMES
}

func (e SystemGraphEdge) IsSharable(originState *GlobalState) (bool, string) {
	return true, ""
}

type SystemGraphEdgeKind uint8

const (
	EdgeChild SystemGraphEdgeKind = iota + 1
	EdgeWatched
)

func (k SystemGraphEdgeKind) DefaultText() string {
	var edgeText string
	switch k {
	case EdgeChild:
		edgeText = DEFAULT_EDGE_TO_CHILD_TEXT
	case EdgeWatched:
		edgeText = DEFAULT_EDGE_TO_WATCHED_CHILD_TEXT
	default:
		panic(ErrUnreachable)
	}
	return edgeText
}

// A SystemGraphEvent is an immutable value representing an event in an node or between two nodes.
type SystemGraphEvent struct {
	value0Ptr, value1Ptr uintptr
	text                 string
	date                 Date

	NotClonableMixin
	NoReprMixin
}

func (e SystemGraphEvent) Prop(ctx *Context, name string) Value {
	switch name {
	case "text":
		return Str(e.text)
	case "value0_id":
		return Int(e.value0Ptr)
	}
	panic(FormatErrPropertyDoesNotExist(name, e))
}

func (SystemGraphEvent) SetProp(ctx *Context, name string, value Value) error {
	return ErrCannotSetProp
}

func (SystemGraphEvent) PropertyNames(ctx *Context) []string {
	return SYSTEM_GRAPH_EVENT_PROPNAMES
}

type SystemGraphNodeValue interface {
	Watchable
	ProposeSystemGraph(ctx *Context, g *SystemGraph, propoposedName string, optionalParent SystemGraphNodeValue)
	SystemGraph() *SystemGraph
	AddSystemGraphEvent(ctx *Context, text string)
}

func (g *SystemGraph) Ptr() SystemGraphPointer {
	return SystemGraphPointer{ptr: unsafe.Pointer(g)}
}

func (g *SystemGraph) AddNode(ctx *Context, value SystemGraphNodeValue, name string) {
	g.nodes.lock.Lock()
	defer g.nodes.lock.Unlock()

	if g.isFrozen {
		panic(ErrAttemptToMutateFrozenValue)
	}

	n := g.addNode(ctx, value, name)
	if n == nil {
		return
	}

	specificMutation := NewSpecificMutation(ctx, SpecificMutationMetadata{
		Version: 1,
		Kind:    SG_AddNode,
		Depth:   ShallowWatching,
	}, Str(n.name), Str(n.typeName), Int(n.valuePtr), Int(0))

	g.mutationCallbacks.CallMicrotasks(ctx, specificMutation)
}

func (g *SystemGraph) addNodeWithEdges(ctx *Context, source SystemGraphNodeValue, value SystemGraphNodeValue, name string, edgeKind SystemGraphEdgeKind) {
	g.nodes.lock.Lock()
	defer g.nodes.lock.Unlock()

	g.addNodeWithEdgesNoLock(ctx, source, value, name, edgeKind)
}

func (g *SystemGraph) addNodeWithEdgesNoLock(
	ctx *Context, parent SystemGraphNodeValue, value SystemGraphNodeValue, name string, edgeKind SystemGraphEdgeKind,
	additionalEdgeKinds ...SystemGraphEdgeKind,
) (pNode *SystemGraphNode, cNode *SystemGraphNode) {
	if g.isFrozen {
		panic(ErrAttemptToMutateFrozenValue)
	}

	parentReflectVal := reflect.ValueOf(parent)
	parentPtr := parentReflectVal.Pointer()
	parentNode, ok := g.nodes.ptrToNode[parentPtr]
	if !ok {
		panic(ErrValueNotInSysGraph)
	}

	childNode := g.addNode(ctx, value, name)
	if childNode == nil {
		return
	}

	edgeText := edgeKind.DefaultText()
	g.addEdgeNoLock(edgeText, childNode, parentNode, edgeKind)

	mutationMetaData := SpecificMutationMetadata{
		Version: 1,
		Kind:    SG_AddNode,
		Depth:   ShallowWatching,
	}
	var specificMutation Mutation

	if len(additionalEdgeKinds) == 0 {
		specificMutation =
			NewSpecificMutation(
				ctx, mutationMetaData, Str(childNode.name), Str(childNode.typeName),
				Int(childNode.valuePtr), Int(parentNode.valuePtr), Str(edgeText), Int(edgeKind))
	} else {
		tupleElements := make([]Value, 2+2*len(additionalEdgeKinds))
		tupleElements[0] = Str(edgeText)
		tupleElements[1] = Int(edgeKind)

		for i, additionalEdgeKind := range additionalEdgeKinds {
			edgeText := additionalEdgeKind.DefaultText()
			g.addEdgeNoLock(edgeText, childNode, parentNode, additionalEdgeKind)
			tupleElements[2+i] = Str(edgeText)
			tupleElements[2+i+1] = Int(additionalEdgeKind)
		}

		specificMutation =
			NewSpecificMutation(
				ctx, mutationMetaData, Str(childNode.name), Str(childNode.typeName),
				Int(childNode.valuePtr), Int(parentNode.valuePtr), NewTuple(tupleElements))
	}

	g.mutationCallbacks.CallMicrotasks(ctx, specificMutation)
	return parentNode, childNode
}

// AddChildNode is like AddNode but it also adds an edge of kind EdgeChild from the parent value's node to the newly created node
func (g *SystemGraph) AddChildNode(ctx *Context, parent SystemGraphNodeValue, value SystemGraphNodeValue, name string, additionalEdgeKinds ...SystemGraphEdgeKind) {

	for _, kind := range additionalEdgeKinds {
		if kind == EdgeChild {
			panic(errors.New("failed to add child node: additional edge kind is EdgeChild"))
		}
	}

	g.nodes.lock.Lock()
	defer g.nodes.lock.Unlock()

	g.addNodeWithEdgesNoLock(ctx, parent, value, name, EdgeChild, additionalEdgeKinds...)
}

// AddWatcheddNode is like AddChildNode but the kind of the newly created edge is EdgeWatched
func (g *SystemGraph) AddWatchedNode(ctx *Context, watchingVal SystemGraphNodeValue, watchedValue SystemGraphNodeValue, name string) {
	g.addNodeWithEdges(ctx, watchingVal, watchedValue, name, EdgeWatched)
}

func (g *SystemGraph) addEdgeWithMutationNoLock(ctx *Context, fromNode, toNode *SystemGraphNode, kind SystemGraphEdgeKind, edgeText string) {
	g.addEdgeNoLock(edgeText, toNode, fromNode, kind)

	specificMutation := NewSpecificMutation(ctx, SpecificMutationMetadata{
		Version: 1,
		Kind:    SG_AddEdge,
		Depth:   ShallowWatching,
	}, Int(fromNode.valuePtr), Int(toNode.valuePtr), Str(edgeText), Int(kind))

	g.mutationCallbacks.CallMicrotasks(ctx, specificMutation)
}

func (g *SystemGraph) addEdgeNoLock(edgeText string, childNode *SystemGraphNode, parentNode *SystemGraphNode, kind SystemGraphEdgeKind) SystemGraphEdge {
	edge := SystemGraphEdge{
		text: edgeText,
		to:   childNode.valuePtr,
		kind: kind,
	}
	parentNode.edgesFrom = append(parentNode.edgesFrom, edge)
	return edge
}

func (g *SystemGraph) addNode(ctx *Context, value SystemGraphNodeValue, name string) *SystemGraphNode {
	reflectVal := reflect.ValueOf(value)
	if reflectVal.Kind() != reflect.Pointer {
		panic(fmt.Errorf("failed to add node to system graph: %w: %#v", ErrValueNotPointer, value))
	}
	ptr := reflectVal.Pointer()

	_, alreadyAdded := g.nodes.ptrToNode[ptr]
	if alreadyAdded {
		return nil
	}

	runtime.SetFinalizer(value, func(v SystemGraphNodeValue) {
		g.nodes.lock.Lock()
		defer g.nodes.lock.Unlock()
		ptr := reflect.ValueOf(v).Pointer()
		node, ok := g.nodes.ptrToNode[ptr]
		if ok {
			node.valuePtr = 0
			node.version = 0
			node.typeName = ""
			node.name = ""
			node.edgesFrom = node.edgesFrom[:0]
			node.available = true

			g.nodes.availableNodes = append(g.nodes.availableNodes, node)
		}
	})

	return g.addNodeNoLock(ctx, ptr, name, reflectVal.Elem().Type().Name())
}

func (g *SystemGraph) addNodeNoLock(ctx *Context, ptr uintptr, name string, typename string) *SystemGraphNode {
	// create the node

	g.lastSnapshot = nil

	var node *SystemGraphNode

	if len(g.nodes.availableNodes) > 0 { // reuse a previous node
		node = g.nodes.availableNodes[len(g.nodes.availableNodes)-1]
		node.available = false
		g.nodes.availableNodes = g.nodes.availableNodes[:len(g.nodes.availableNodes)-1]
	} else {
		node = new(SystemGraphNode)
		g.nodes.list = append(g.nodes.list, node)
	}

	*node = SystemGraphNode{
		valuePtr: ptr,
		name:     name,
		typeName: typename,
		index:    len(g.nodes.list),
	}

	g.nodes.ptrToNode[ptr] = node
	return node
}

func (g *SystemGraph) AddEvent(ctx *Context, text string, v SystemGraphNodeValue) {
	ptr := reflect.ValueOf(v).Pointer()

	g.nodes.lock.Lock()
	if g.isFrozen {
		panic(ErrAttemptToMutateFrozenValue)
	}

	node, ok := g.nodes.ptrToNode[ptr]
	g.nodes.lock.Unlock()

	if !ok {
		panic(ErrValueNotInSysGraph)
	}

	g.eventLogLock.Lock()
	defer g.eventLogLock.Unlock()

	g.addEventNoLock(node.valuePtr, text)

	specificMutation := NewSpecificMutation(ctx, SpecificMutationMetadata{
		Version: 1,
		Kind:    SG_AddEvent,
		Depth:   ShallowWatching,
	}, Int(node.valuePtr), Str(text))

	g.mutationCallbacks.CallMicrotasks(ctx, specificMutation)
}

func (g *SystemGraph) addEventNoLock(nodeValuePtr uintptr, text string) {
	g.lastSnapshot = nil

	g.eventLog = append(g.eventLog, SystemGraphEvent{
		value0Ptr: nodeValuePtr,
		text:      text,
	})
}

type SystemGraphPointer struct{ ptr unsafe.Pointer }

func (p *SystemGraphPointer) Graph() *SystemGraph {
	return (*SystemGraph)(unsafe.Pointer(p.ptr))
}

func (p *SystemGraphPointer) Set(ptr SystemGraphPointer) bool {
	if uintptr(p.ptr) != 0 {
		return false
		//panic(ErrValueAlreadyInSysGraph)
	}
	p.ptr = ptr.ptr
	return true
}

func (p *SystemGraphPointer) AddEvent(ctx *Context, text string, v SystemGraphNodeValue) {
	if uintptr(p.ptr) == 0 {
		return
	}
	p.Graph().AddEvent(ctx, text, v)
}

func (g *SystemGraph) Prop(ctx *Context, name string) Value {
	switch name {
	case "nodes":
		return g.nodes
	case "events":
		g.eventLogLock.Lock()
		defer g.eventLogLock.Unlock()

		//TODO: refactor
		events := make([]Value, len(g.eventLog))
		for i, e := range g.eventLog {
			events[i] = e
		}
		return NewTuple(events)
	}
	panic(FormatErrPropertyDoesNotExist(name, g))
}

func (*SystemGraph) SetProp(ctx *Context, name string, value Value) error {
	return ErrCannotSetProp
}

func (*SystemGraph) PropertyNames(ctx *Context) []string {
	return SYSTEM_GRAPH_PROPNAMES
}

func (g *SystemGraph) IsSharable(originState *GlobalState) (bool, string) {
	return true, ""
}

func (g *SystemGraph) Share(originState *GlobalState) {

}

func (g *SystemGraph) IsShared() bool {
	return true
}

func (g *SystemGraph) ForceLock() {

}

func (g *SystemGraph) ForceUnlock() {

}

type SystemGraphNodes struct {
	lock           sync.Mutex
	list           []*SystemGraphNode
	ptrToNode      map[uintptr]*SystemGraphNode
	availableNodes []*SystemGraphNode //TODO: replace with a bitset
	graph          *SystemGraph

	NoReprMixin
	NotClonableMixin
}

func (n *SystemGraphNodes) IsSharable(originState *GlobalState) (bool, string) {
	return true, ""
}

func (n *SystemGraphNodes) Share(originState *GlobalState) {

}

func (n *SystemGraphNodes) IsShared() bool {
	return true
}

func (n *SystemGraphNodes) ForceLock() {

}

func (n *SystemGraphNodes) ForceUnlock() {

}

type SystemGraphNode struct {
	valuePtr  uintptr
	name      string
	typeName  string
	index     int
	edgesFrom []SystemGraphEdge
	available bool
	version   uint64

	NoReprMixin
	NotClonableMixin
}

func (n *SystemGraphNode) Prop(ctx *Context, name string) Value {
	switch name {
	case "name":
		return Str(n.name)
	case "type_name":
		return Str(n.typeName)
	case "value_id":
		return Int(n.valuePtr)
	case "edges":
		values := make([]Value, len(n.edgesFrom))
		for i, e := range n.edgesFrom {
			values[i] = e
		}
		return NewTuple(values)
	}
	panic(FormatErrPropertyDoesNotExist(name, n))
}

func (*SystemGraphNode) SetProp(ctx *Context, name string, value Value) error {
	return ErrCannotSetProp
}

func (*SystemGraphNode) PropertyNames(ctx *Context) []string {
	return SYSTEM_GRAPH_NODE_PROPNAMES
}

//TODO: lock

func (n *SystemGraphNode) IsSharable(originState *GlobalState) (bool, string) {
	return true, ""
}

func (n *SystemGraphNode) Share(originState *GlobalState) {

}

func (n *SystemGraphNode) IsShared() bool {
	return true
}

func (n *SystemGraphNode) ForceLock() {

}

func (n *SystemGraphNode) ForceUnlock() {

}

func (obj *Object) ProposeSystemGraph(ctx *Context, g *SystemGraph, proposedName string, optionalParent SystemGraphNodeValue) {
	state := ctx.GetClosestState()

	obj.lock.Lock(state, obj)
	defer obj.lock.Unlock(state, obj)

	ptr := g.Ptr()
	if !obj.sysgraph.Set(ptr) {
		return
	}

	if optionalParent == nil {
		g.AddNode(ctx, obj, proposedName)
	} else {
		g.AddChildNode(ctx, optionalParent, obj, proposedName)
	}

	for _, part := range obj.systemParts {

		key := ""
		for i, k := range obj.keys {
			v := obj.values[i]
			if Same(part, v) {
				key = k
				break
			}
		}

		part.ProposeSystemGraph(ctx, g, key, obj)
	}
}

func (obj *Object) SystemGraph() *SystemGraph {
	return obj.sysgraph.Graph()
}

func (obj *Object) AddSystemGraphEvent(ctx *Context, text string) {
	obj.sysgraph.AddEvent(ctx, text, obj)
}
