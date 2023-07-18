package html_ns

import (
	"bufio"

	"github.com/inoxlang/inox/internal/core/symbolic"
	pprint "github.com/inoxlang/inox/internal/pretty_print"

	"github.com/inoxlang/inox/internal/utils"
)

type HTMLNode struct {
	symbolic.UnassignablePropsMixin
	symbolic.SerializableMixin
}

func NewHTMLNode() *HTMLNode {
	return &HTMLNode{}
}

func (n *HTMLNode) Test(v symbolic.SymbolicValue) bool {
	_, ok := v.(*HTMLNode)
	if !ok {
		return false
	}

	return true
}

func (n *HTMLNode) Prop(name string) symbolic.SymbolicValue {
	switch name {
	case "first-child":
		return NewHTMLNode()
	case "data":
		return &symbolic.String{}
	default:
		method, ok := n.GetGoMethod(name)
		if !ok {
			panic(symbolic.FormatErrPropertyDoesNotExist(name, n))
		}
		return method
	}
}

func (n *HTMLNode) GetGoMethod(name string) (*symbolic.GoFunction, bool) {
	return nil, false
}

func (n *HTMLNode) PropertyNames() []string {
	return []string{"first-child", "data"}
}

func (r *HTMLNode) Widen() (symbolic.SymbolicValue, bool) {
	return nil, false
}

func (r *HTMLNode) IsWidenable() bool {
	return false
}

func (r *HTMLNode) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%html-node")))
	return
}

func (r *HTMLNode) WidestOfType() symbolic.SymbolicValue {
	return &HTMLNode{}
}
