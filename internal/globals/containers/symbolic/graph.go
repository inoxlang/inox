package containers

import (
	"github.com/inoxlang/inox/internal/core/symbolic"
	pprint "github.com/inoxlang/inox/internal/pretty_print"
)

var (
	_         = []symbolic.Iterable{(*Graph)(nil)}
	ANY_GRAPH = &Graph{}

	_ = symbolic.IProps((*Graph)(nil))
	_ = symbolic.IProps((*GraphNode)(nil))
)

type Graph struct {
	symbolic.UnassignablePropsMixin
	_ int
}

func (r *Graph) Test(v symbolic.Value, state symbolic.RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	_, ok := v.(*Graph)
	return ok
}

func (f *Graph) GetGoMethod(name string) (*symbolic.GoFunction, bool) {
	switch name {
	case "insert_node":
		return symbolic.WrapGoMethod(f.InsertNode), true
	case "remove_node":
		return symbolic.WrapGoMethod(f.RemoveNode), true
	case "connect":
		return symbolic.WrapGoMethod(f.Connect), true
	}
	return nil, false
}

func (g *Graph) Prop(name string) symbolic.Value {
	return symbolic.GetGoMethodOrPanic(name, g)
}

func (*Graph) PropertyNames() []string {
	return []string{"insert_node", "remove_node", "connect"}
}

func (f *Graph) InsertNode(ctx *symbolic.Context, v symbolic.Value) *GraphNode {
	return &GraphNode{}
}

func (f *Graph) RemoveNode(ctx *symbolic.Context, k *GraphNode) {

}

func (f *Graph) Connect(ctx *symbolic.Context, n1, n2 *GraphNode) {

}

func (f *Graph) Get(ctx *symbolic.Context, k symbolic.Value) symbolic.Value {
	return symbolic.ANY
}

func (r *Graph) PrettyPrint(w symbolic.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("graph")
}

func (g *Graph) IteratorElementKey() symbolic.Value {
	return symbolic.ANY
}

func (r *Graph) IteratorElementValue() symbolic.Value {
	return symbolic.ANY
}

func (r *Graph) WalkerElement() symbolic.Value {
	return &GraphNode{}
}

func (r *Graph) WalkerNodeMeta() symbolic.Value {
	return symbolic.ANY
}

func (r *Graph) WidestOfType() symbolic.Value {
	return ANY_GRAPH
}

type GraphNode struct {
	symbolic.UnassignablePropsMixin
	_ int
}

func (r *GraphNode) Test(v symbolic.Value, state symbolic.RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	_, ok := v.(*GraphNode)
	return ok
}

func (f *GraphNode) GetGoMethod(name string) (*symbolic.GoFunction, bool) {
	switch name {
	}
	return nil, false
}

func (n *GraphNode) Prop(name string) symbolic.Value {
	switch name {
	case "data":
		return symbolic.ANY
	case "children":
		return &symbolic.Iterator{ElementValue: &GraphNode{}}
	case "parents":
		return &symbolic.Iterator{ElementValue: &GraphNode{}}
	}
	return symbolic.GetGoMethodOrPanic(name, n)
}

func (*GraphNode) PropertyNames() []string {
	return []string{"data", "children", "parents"}
}

func (r *GraphNode) PrettyPrint(w symbolic.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("graph-node")
}

func (r *GraphNode) WidestOfType() symbolic.Value {
	return &GraphNode{}
}
