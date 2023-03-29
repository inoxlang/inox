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
	SG_AddNode SpecificMutationKind = iota + 1
	SG_AddEvent
)

var (
	ErrValueAlreadyInSysGraph = errors.New("value already in a system graph")
	ErrValueNotInSysGraph     = errors.New("value is not part of system graph")
	ErrValueNotPointer        = errors.New("value is not a pointer")

	SYSTEM_GRAPH_PROPNAMES       = []string{"nodes", "events"}
	SYSTEM_GRAPH_EVENT_PROPNAMES = []string{"text"}
	SYSTEM_GRAPH_NODE_PROPNAMES  = []string{"name", "type_name", "value_id"}

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
	to   *SystemGraphNode
}

type SystemGraphEdgeKind uint8

// A SystemGraphEvent is an immutable value representing an event in an node or between two nodes.
type SystemGraphEvent struct {
	node0, node1 uintptr
	text         string
	date         Date

	NotClonableMixin
	NoReprMixin
}

func (e SystemGraphEvent) Prop(ctx *Context, name string) Value {
	switch name {
	case "text":
		return Str(e.text)
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
	ProposeSystemGraph(ctx *Context, g *SystemGraph, propoposedName string)
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

	reflectVal := reflect.ValueOf(value)
	if reflectVal.Kind() != reflect.Pointer {
		panic(fmt.Errorf("failed to add node to system graph: %w: %#v", ErrValueNotPointer, value))
	}
	ptr := reflectVal.Pointer()

	_, alreadyAdded := g.nodes.ptrToNode[ptr]
	if alreadyAdded {
		return
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
			//note: we don't change the index

			g.nodes.availableNodes = append(g.nodes.availableNodes, node)
		}
	})

	n := g.addNodeNoLock(ctx, ptr, name, reflectVal.Elem().Type().Name())

	specificMutation := NewSpecificMutation(ctx, SpecificMutationMetadata{
		Version: 1,
		Kind:    SG_AddNode,
		Depth:   ShallowWatching,
	}, Str(n.name), Str(n.typeName), Int(n.valuePtr))

	g.mutationCallbacks.CallMicrotasks(ctx, specificMutation)
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
		node0: nodeValuePtr,
		text:  text,
	})
}

type SystemGraphPointer struct{ ptr unsafe.Pointer }

func (p *SystemGraphPointer) Graph() *SystemGraph {
	return (*SystemGraph)(unsafe.Pointer(p.ptr))
}

func (p *SystemGraphPointer) Set(ptr SystemGraphPointer) {
	if uintptr(p.ptr) != 0 {
		panic(ErrValueAlreadyInSysGraph)
	}
	p.ptr = ptr.ptr
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

//

func (obj *Object) ProposeSystemGraph(ctx *Context, g *SystemGraph, proposedName string) {
	ptr := g.Ptr()
	obj.sysgraph.Set(ptr)

	g.AddNode(ctx, obj, proposedName)
}

func (obj *Object) SystemGraph() *SystemGraph {
	return obj.sysgraph.Graph()
}

func (obj *Object) AddSystemGraphEvent(ctx *Context, text string) {
	obj.sysgraph.AddEvent(ctx, text, obj)
}
