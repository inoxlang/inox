package containers

import (
	"bufio"

	symbolic "github.com/inoxlang/inox/internal/core/symbolic"
	pprint "github.com/inoxlang/inox/internal/pretty_print"

	"github.com/inoxlang/inox/internal/utils"
)

var _ = []symbolic.Iterable{&Graph{}}

type Graph struct {
	symbolic.UnassignablePropsMixin
	_ int
}

func (r *Graph) Test(v symbolic.SymbolicValue) bool {
	_, ok := v.(*Graph)
	return ok
}

func (r Graph) Clone(clones map[uintptr]symbolic.SymbolicValue) symbolic.SymbolicValue {
	return &Graph{}
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

func (g *Graph) Prop(name string) symbolic.SymbolicValue {
	return symbolic.GetGoMethodOrPanic(name, g)
}

func (*Graph) PropertyNames() []string {
	return []string{"insert_node", "remove_node", "connect"}
}

func (f *Graph) InsertNode(ctx *symbolic.Context, v symbolic.SymbolicValue) *GraphNode {
	return &GraphNode{}
}

func (f *Graph) RemoveNode(ctx *symbolic.Context, k *GraphNode) {

}

func (f *Graph) Connect(ctx *symbolic.Context, n1, n2 *GraphNode) {

}

func (f *Graph) Get(ctx *symbolic.Context, k symbolic.SymbolicValue) symbolic.SymbolicValue {
	return &symbolic.Any{}
}

func (r *Graph) Widen() (symbolic.SymbolicValue, bool) {
	return nil, false
}

func (a *Graph) IsWidenable() bool {
	return false
}

func (r *Graph) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%graph")))
	return
}

func (g *Graph) IteratorElementKey() symbolic.SymbolicValue {
	return &symbolic.Any{}
}

func (r *Graph) IteratorElementValue() symbolic.SymbolicValue {
	return &symbolic.Any{}
}

func (r *Graph) WalkerElement() symbolic.SymbolicValue {
	return &GraphNode{}
}

func (r *Graph) WalkerNodeMeta() symbolic.SymbolicValue {
	return symbolic.ANY
}

func (r *Graph) WidestOfType() symbolic.SymbolicValue {
	return &Graph{}
}

type GraphNode struct {
	_ int
}

func (r *GraphNode) Test(v symbolic.SymbolicValue) bool {
	_, ok := v.(*GraphNode)
	return ok
}

func (r GraphNode) Clone(clones map[uintptr]symbolic.SymbolicValue) symbolic.SymbolicValue {
	return &GraphNode{}
}

func (f *GraphNode) GetGoMethod(name string) (*symbolic.GoFunction, bool) {
	switch name {
	}
	return nil, false
}

func (n *GraphNode) Prop(name string) symbolic.SymbolicValue {
	switch name {
	case "data":
		return &symbolic.Any{}
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

func (r *GraphNode) Widen() (symbolic.SymbolicValue, bool) {
	return nil, false
}

func (a *GraphNode) IsWidenable() bool {
	return false
}

func (r *GraphNode) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%graph-node")))
	return
}

func (r *GraphNode) WidestOfType() symbolic.SymbolicValue {
	return &GraphNode{}
}
