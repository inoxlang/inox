package html_ns

import (
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/prettyprint"
	pprint "github.com/inoxlang/inox/internal/prettyprint"
)

var (
	HTML_NODE_PROPNAMES = []string{"first-child", "data"}

	_             symbolic.Watchable = (*HTMLNode)(nil)
	ANY_HTML_NODE                    = &HTMLNode{}
)

type HTMLNode struct {
	symbolic.UnassignablePropsMixin
	symbolic.SerializableMixin
}

func NewHTMLNode() *HTMLNode {
	return &HTMLNode{}
}

func (n *HTMLNode) Test(v symbolic.Value, state symbolic.RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	_, ok := v.(*HTMLNode)
	if !ok {
		return false
	}

	return true
}

func (n *HTMLNode) Prop(name string) symbolic.Value {
	switch name {
	case "first-child":
		return NewHTMLNode()
	case "data":
		return symbolic.ANY_STR
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
	return HTML_NODE_PROPNAMES
}

func (n *HTMLNode) WatcherElement() symbolic.Value {
	return symbolic.ANY
}

func (n *HTMLNode) PrettyPrint(w prettyprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("html-node")
}

func (r *HTMLNode) WidestOfType() symbolic.Value {
	return &HTMLNode{}
}
