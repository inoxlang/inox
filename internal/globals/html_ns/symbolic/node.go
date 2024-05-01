package html_ns

import (
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/prettyprint"
	pprint "github.com/inoxlang/inox/internal/prettyprint"
)

var (
	HTML_NODE_PROPNAMES = []string{"first-child", "data"}
	ANY_HTML_NODE       = &HTMLNode{}

	_ symbolic.Watchable  = (*HTMLNode)(nil)
	_ symbolic.MarkupNode = (*HTMLNode)(nil)
)

// An HTMLNode represents a symbolic HTMLNode.
type HTMLNode struct {
	tagName            string //empty if any tag name is matched
	requiredAttributes []HTMLAttribute
	requiredChildren   []*HTMLNode //order is irrelevant, repetitions are ignored
	symbolic.UnassignablePropsMixin
	symbolic.SerializableMixin
	symbolic.MarkupNodeMixin
}

func (n *HTMLNode) Test(v symbolic.Value, state symbolic.RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	otherNode, ok := v.(*HTMLNode)
	if !ok {
		return false
	}

	//Check tag name.

	if n.tagName != "" && otherNode.tagName != n.tagName {
		return false
	}

	//Check attributes.

	for _, attr := range n.requiredAttributes {
		for _, otherNodeAttr := range otherNode.requiredAttributes {
			if otherNodeAttr.name == attr.name && !attr.stringValue.Test(otherNodeAttr.stringValue, state) {
				return false
			}
		}
	}

	//Check children.

check_children:
	for _, child := range n.requiredChildren {
		for _, otherNodeChild := range otherNode.requiredChildren {
			if child.Test(otherNodeChild, state) {
				continue check_children
			}
		}
		//No child of the other node matches $child.
		return false
	}

	return true
}

func (n *HTMLNode) Prop(name string) symbolic.Value {
	switch name {
	case "first-child":
		return ANY_HTML_NODE
	case "data":
		return symbolic.ANY_STRING
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

func (n *HTMLNode) WidestOfType() symbolic.Value {
	return ANY_HTML_NODE
}

// An HTMLAttribute represents a symbolic HTMLAttribute.
type HTMLAttribute struct {
	name        string
	stringValue *symbolic.String
}

func NewHTMLAttribute(name string, value *symbolic.String) HTMLAttribute {
	return HTMLAttribute{
		name:        name,
		stringValue: value,
	}
}
