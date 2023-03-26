package internal

import (
	"errors"
	"fmt"
	"reflect"
	"runtime"
	"sync"
	"unsafe"
)

var (
	ErrValueAlreadyInSysGraph = errors.New("value already in a system graph")
	ErrValueNotInSysGraph     = errors.New("value is not part of system graph")
)

// A SystemGraph represents relations & events between values.
type SystemGraph struct {
	nodes *SystemGraphNodes

	eventLogLock sync.Mutex
	eventLog     []SystemGraphEvent
}

func NewSystemGraph() *SystemGraph {
	g := &SystemGraph{
		nodes: &SystemGraphNodes{
			ptrToNode: make(map[uintptr]*SystemGraphNode),
		},
	}

	return g
}

type SystemGraphNodes struct {
	lock           sync.Mutex
	ptrToNode      map[uintptr]*SystemGraphNode
	availableNodes []*SystemGraphNode
}

type SystemGraphNode struct {
	valuePtr  uintptr
	name      string
	edgesFrom []SystemGraphEdge
	available bool
	version   uint64
}

type SystemGraphEdge struct {
	text string
	to   *SystemGraphNode
}

type SystemGraphEdgeKind uint8

type SystemGraphEvent struct {
	node0, node1 *SystemGraphNode
	otherNodes   []SystemGraphNode
	text         string
}

type SystemGraphNodeValue interface {
	Watchable
	ProposeSystemGraph(g *SystemGraph)
}

func (g *SystemGraph) Ptr() SystemGraphPointer {
	return SystemGraphPointer{ptr: unsafe.Pointer(g)}
}

func (g *SystemGraph) AddNode(value SystemGraphNodeValue) {
	g.nodes.lock.Lock()
	defer g.nodes.lock.Unlock()

	reflectVal := reflect.ValueOf(value)
	if reflectVal.Kind() != reflect.Pointer {
		panic(fmt.Errorf("failed to add node to system graph, following value is not a pointer: %#v", value))
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
			node.name = ""
			node.edgesFrom = node.edgesFrom[:0]
			node.available = true
			delete(g.nodes.ptrToNode, ptr)

			g.nodes.availableNodes = append(g.nodes.availableNodes, node)
		}
	})

	// create the node

	var node *SystemGraphNode

	if len(g.nodes.availableNodes) > 0 { // reuse a previous node
		node = g.nodes.availableNodes[len(g.nodes.availableNodes)-1]
		g.nodes.availableNodes = g.nodes.availableNodes[:len(g.nodes.availableNodes)-1]
	} else {
		node = &SystemGraphNode{}
	}

	*node = SystemGraphNode{
		valuePtr: ptr,
		name:     reflectVal.Elem().Type().Name(),
	}

	g.nodes.ptrToNode[ptr] = node
}

func (g *SystemGraph) AddEvent(text string, v SystemGraphNodeValue) {
	ptr := reflect.ValueOf(v).Pointer()

	g.nodes.lock.Lock()
	node, ok := g.nodes.ptrToNode[ptr]
	g.nodes.lock.Unlock()

	if !ok {
		panic(ErrValueNotInSysGraph)
	}

	g.eventLogLock.Lock()
	defer g.eventLogLock.Unlock()

	g.eventLog = append(g.eventLog, SystemGraphEvent{
		node0: node,
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

func (p *SystemGraphPointer) AddEvent(text string, v SystemGraphNodeValue) {
	if uintptr(p.ptr) == 0 {
		return
	}
	p.Graph().AddEvent(text, v)
}

//

func (obj *Object) ProposeSystemGraph(g *SystemGraph) {
	ptr := g.Ptr()
	obj.sysgraph.Set(ptr)

	g.AddNode(obj)
}
