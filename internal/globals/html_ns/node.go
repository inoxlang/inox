package html_ns

import (
	"io"

	core "github.com/inoxlang/inox/internal/core"
	jsoniter "github.com/json-iterator/go"
	"golang.org/x/net/html"
)

var _ = []core.GoValue{&HTMLNode{}}

type HTMLNode struct {
	node         *html.Node // TODO: make private
	render       []byte
	cloneOnWrite bool
}

func NewHTMLNode(n *html.Node) *HTMLNode {
	return &HTMLNode{node: n}
}

func (n *HTMLNode) Prop(ctx *core.Context, name string) core.Value {
	switch name {
	case "first-child":
		return &HTMLNode{node: n.node.FirstChild}
	case "data":
		return core.Str(n.node.Data)
	default:
		method, ok := n.GetGoMethod(name)
		if !ok {
			panic(core.FormatErrPropertyDoesNotExist(name, n))
		}
		return method
	}
}

func (*HTMLNode) SetProp(ctx *core.Context, name string, value core.Value) error {
	return core.ErrCannotSetProp
}

func (n *HTMLNode) GetGoMethod(name string) (*core.GoFunction, bool) {
	return nil, false
}

func (n *HTMLNode) PropertyNames(ctx *core.Context) []string {
	return []string{"first-child", "data"}
}

func (n *HTMLNode) IsRecursivelyRenderable(ctx *core.Context, input core.RenderingInput) bool {
	return input.Mime == core.HTML_CTYPE
}

//

func (n *HTMLNode) DiscardCache() {
	n.render = nil
}

func (n *HTMLNode) replaceByClone() map[*html.Node]*html.Node {
	clones := map[*html.Node]*html.Node{}
	n.node = cloneHtmlNode(n.node, clones)
	return clones
}

func (n *HTMLNode) Data() string {
	return n.node.Data
}

func (n *HTMLNode) HasParent() bool {
	return n.node.Parent != nil
}

func (n *HTMLNode) HasPrevSibling() bool {
	return n.node.PrevSibling != nil
}

func (n *HTMLNode) HasNextSibling() bool {
	return n.node.NextSibling != nil
}

func (n *HTMLNode) HasId() bool {
	for _, attr := range n.node.Attr {
		if attr.Key == "id" {
			return true
		}
	}
	return false
}

func (n *HTMLNode) Attr(name string) (string, bool) {
	for _, attr := range n.node.Attr {
		if attr.Key == name {
			return attr.Val, true
		}
	}
	return "", false
}

func (n *HTMLNode) AttrOrEmpty(name string) string {
	for _, attr := range n.node.Attr {
		if attr.Key == name {
			return attr.Val
		}
	}
	return ""
}

func (n *HTMLNode) Walk(fn func(n HTMLNode) error) error {
	if err := fn(*n); err != nil {
		return err
	}

	child := n.node.FirstChild
	for child != nil {
		node := HTMLNode{node: child}
		if err := node.Walk(fn); err != nil {
			return err
		}
		child = child.NextSibling
	}

	return nil
}

func (n *HTMLNode) ReplaceChildHTML(ctx *core.Context, prevHTMLNode *HTMLNode, child *HTMLNode) {
	newHTMLnode := child.node
	current := n.node.FirstChild

	for current != nil && current != prevHTMLNode.node {
		current = current.NextSibling
	}

	if current == nil { // prev child not found
		return
	}

	n.DiscardCache()

	if n.cloneOnWrite {
		n.cloneOnWrite = false
		clones := n.replaceByClone()
		current = clones[current]
	}

	if current.PrevSibling != nil {
		current.PrevSibling.NextSibling = newHTMLnode
		newHTMLnode.PrevSibling = current.PrevSibling
	}

	if current.NextSibling != nil {
		current.NextSibling.PrevSibling = newHTMLnode
		newHTMLnode.NextSibling = current.NextSibling
	}

	if current == n.node.FirstChild {
		n.node.FirstChild = newHTMLnode
	}

}

func (n *HTMLNode) SetAttribute(ctx *core.Context, newAttr html.Attribute) {
	n.DiscardCache()

	if n.cloneOnWrite {
		n.cloneOnWrite = false
		n.replaceByClone()
	}

	for _, attr := range n.node.Attr {
		if attr.Key == newAttr.Key {
			attr.Val = newAttr.Val
			return
		}
	}

	n.node.Attr = append(n.node.Attr, newAttr)
}

func (n *HTMLNode) AppendToAttribute(ctx *core.Context, newAttr html.Attribute) {
	n.DiscardCache()

	if n.cloneOnWrite {
		n.cloneOnWrite = false
		n.replaceByClone()
	}

	for _, attr := range n.node.Attr {
		if attr.Key == newAttr.Key {
			attr.Val = attr.Val + newAttr.Val
			return
		}
	}

	n.node.Attr = append(n.node.Attr, newAttr)
}

func (n *HTMLNode) SetId(ctx *core.Context, id core.Str) {
	n.SetAttribute(ctx, html.Attribute{Key: "id", Val: string(id)})
}

func (n *HTMLNode) RemoveAttribute(ctx *core.Context, name string) {
	n.DiscardCache()

	if n.cloneOnWrite {
		n.cloneOnWrite = false
		n.replaceByClone()
	}

	for i, attr := range n.node.Attr {
		if attr.Key == name {
			if i == len(n.node.Attr)-1 {
				n.node.Attr = n.node.Attr[:len(n.node.Attr)-1]
			} else {
				copy(n.node.Attr[i:], n.node.Attr[i+1:])
				n.node.Attr = n.node.Attr[:len(n.node.Attr)-1]
			}
			if len(n.node.Attr) == 0 {
				n.node.Attr = nil
			}
			return
		}
	}

}

func (n *HTMLNode) WriteRepresentation(ctx *core.Context, w io.Writer, config *core.ReprConfig, depth int) error {
	return core.ErrNotImplementedYet
}

func (n *HTMLNode) WriteJSONRepresentation(ctx *core.Context, w *jsoniter.Stream, config core.JSONSerializationConfig, depth int) error {
	return core.ErrNotImplementedYet
}
