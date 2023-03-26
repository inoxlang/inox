package internal

import "errors"

var (
	ANY_SYSTEM_GRAPH       = NewSystemGraph()
	ANY_SYSTEM_GRAPH_NODES = NewSystemGraphNodes()
	ANY_SYSTEM_GRAPH_NODE  = NewSystemGraphNode()
	SYSTEM_GRAPH_PROPNAMES = []string{"nodes"}

	_ = []Iterable{(*SystemGraphNodes)(nil)}
)

// An SystemGraph represents a symbolic SystemGraph.
type SystemGraph struct {
	_ int
}

func NewSystemGraph() *SystemGraph {
	return &SystemGraph{}
}

func (g *SystemGraph) Test(v SymbolicValue) bool {
	other, ok := v.(*SystemGraph)
	if ok {
		return true
	}
	_ = other
	return false
}

func (g *SystemGraph) Prop(memberName string) SymbolicValue {
	switch memberName {
	case "nodes":
		return ANY_SYSTEM_GRAPH_NODES
	}
	panic(FormatErrPropertyDoesNotExist(memberName, g))
}

func (d *SystemGraph) SetProp(name string, value SymbolicValue) (IProps, error) {
	return nil, errors.New(FmtCannotAssignPropertyOf(d))
}

func (d *SystemGraph) WithExistingPropReplaced(name string, value SymbolicValue) (IProps, error) {
	return nil, errors.New(FmtCannotAssignPropertyOf(d))
}

func (d *SystemGraph) PropertyNames() []string {
	return SYSTEM_GRAPH_PROPNAMES
}

func (d *SystemGraph) Widen() (SymbolicValue, bool) {
	return nil, false
}

func (d *SystemGraph) IsWidenable() bool {
	return false
}

func (d *SystemGraph) String() string {
	return "system-graph"
}

func (d *SystemGraph) WidestOfType() SymbolicValue {
	return ANY_SYSTEM_GRAPH
}

// An SystemGraphNodes represents a symbolic SystemGraphNodes.
type SystemGraphNodes struct {
	_ int
}

func NewSystemGraphNodes() *SystemGraphNodes {
	return &SystemGraphNodes{}
}

func (g *SystemGraphNodes) Test(v SymbolicValue) bool {
	other, ok := v.(*SystemGraphNodes)
	if ok {
		return true
	}
	_ = other
	return false
}

func (d *SystemGraphNodes) Widen() (SymbolicValue, bool) {
	return nil, false
}

func (d *SystemGraphNodes) IsWidenable() bool {
	return false
}

func (d *SystemGraphNodes) IteratorElementKey() SymbolicValue {
	return ANY
}
func (d *SystemGraphNodes) IteratorElementValue() SymbolicValue {
	return ANY_SYSTEM_GRAPH_NODE
}

func (d *SystemGraphNodes) String() string {
	return "system-graph-nodes"
}

func (d *SystemGraphNodes) WidestOfType() SymbolicValue {
	return ANY_SYSTEM_GRAPH_NODES
}

// An SystemGraphNode represents a symbolic SystemGraphNode.
type SystemGraphNode struct {
	_ int
}

func NewSystemGraphNode() *SystemGraphNode {
	return &SystemGraphNode{}
}

func (g *SystemGraphNode) Test(v SymbolicValue) bool {
	other, ok := v.(*SystemGraphNode)
	if ok {
		return true
	}
	_ = other
	return false
}

func (d *SystemGraphNode) Widen() (SymbolicValue, bool) {
	return nil, false
}

func (d *SystemGraphNode) IsWidenable() bool {
	return false
}

func (d *SystemGraphNode) String() string {
	return "system-graph-node"
}

func (d *SystemGraphNode) WidestOfType() SymbolicValue {
	return ANY_SYSTEM_GRAPH_NODE
}
