package dom_ns

import (
	"bufio"

	symbolic "github.com/inoxlang/inox/internal/core/symbolic"
	pprint "github.com/inoxlang/inox/internal/pretty_print"

	"github.com/inoxlang/inox/internal/utils"
)

type Node struct {
	model symbolic.SymbolicValue
	symbolic.UnassignablePropsMixin
}

func NewDomNode(model symbolic.SymbolicValue) *Node {
	return &Node{model: model}
}

func (n *Node) Test(v symbolic.SymbolicValue) bool {
	otherNode, ok := v.(*Node)
	if !ok {
		return false
	}

	return n.model.Test(otherNode.model)
}

func (n *Node) Prop(name string) symbolic.SymbolicValue {
	switch name {
	case "first-child":
		return NewDomNode(symbolic.Nil)
	case "data":
		return &symbolic.String{}
	case "model":
		return n.model
	default:
		method, ok := n.GetGoMethod(name)
		if !ok {
			panic(symbolic.FormatErrPropertyDoesNotExist(name, n))
		}
		return method
	}
}

func (n *Node) GetGoMethod(name string) (*symbolic.GoFunction, bool) {
	return nil, false
}

func (n *Node) PropertyNames() []string {
	return []string{"first-child", "data", "model"}
}

func (r *Node) Widen() (symbolic.SymbolicValue, bool) {
	return nil, false
}

func (r *Node) IsWidenable() bool {
	return false
}

func (r *Node) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%dom-node")))
	return
}

func (r *Node) WidestOfType() symbolic.SymbolicValue {
	return &Node{}
}
