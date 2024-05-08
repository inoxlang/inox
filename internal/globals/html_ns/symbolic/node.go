package html_ns

import (
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/inoxconsts"
	"github.com/inoxlang/inox/internal/parse"
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
	requiredChildren   []*HTMLNode                //order is irrelevant, repetitions are ignored
	sourceNode         *symbolic.MarkupSourceNode //optional, not used for matching

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

func (n *HTMLNode) SourceNode() (*symbolic.MarkupSourceNode, bool) {
	if n.sourceNode == nil {
		return nil, false
	}
	return n.sourceNode, true
}

// FindNode searches for the node of span $nodeSpan located in the chunk $chunkName by looking at the HTMLNode
// and its descendants.
func (n *HTMLNode) FindNode(nodeSpan parse.NodeSpan, chunkName string) (*HTMLNode, bool) {
	if n.sourceNode != nil && n.sourceNode.Node.Span == nodeSpan && n.sourceNode.Chunk.Name() == chunkName {
		return n, true
	}
	for _, child := range n.requiredChildren {
		foundNode, ok := child.FindNode(nodeSpan, chunkName)
		if ok {
			return foundNode, true
		}
	}
	return nil, false
}

// The result should not be modified.
func (n *HTMLNode) RequiredAttributes() []HTMLAttribute {
	return n.requiredAttributes[0:len(n.requiredAttributes):len(n.requiredAttributes)]
}

func (n *HTMLNode) HyperscriptAttributeValue() (string, bool) {
	for _, attr := range n.requiredAttributes {
		if attr.name == inoxconsts.HYPERSCRIPT_ATTRIBUTE_NAME {
			if attr.stringValue.HasValue() {
				return attr.stringValue.Value(), true
			}
			return "", false
		}
	}
	return "", false
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

func (a HTMLAttribute) Name() string {
	return a.name
}

func (a HTMLAttribute) Value() *symbolic.String {
	return a.stringValue
}
