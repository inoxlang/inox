package html_ns

import (
	"sync"

	"github.com/inoxlang/inox/internal/core"
	"golang.org/x/net/html"
)

var (
	immutableNodePool = &sync.Pool{
		New: func() any {
			return &ImmutableHTMLNode{}
		},
	}
	_ = core.ImmutableMarkupNode((*ImmutableHTMLNode)(nil))
	_ = core.MarkupNode((*HTMLNode)(nil))
)

// An ImmutableHTMLNode is thin wrapper arround an *html.Node. It implements core.ImmutableHTMLNode.
type ImmutableHTMLNode struct {
	node *html.Node
}

func (n *HTMLNode) ImmutableMarkupNode() (core.ImmutableMarkupNode, *sync.Pool) {
	n.cloneOnWrite = true
	immutableNode := immutableNodePool.Get().(*ImmutableHTMLNode)
	immutableNode.node = n.node

	return immutableNode, immutableNodePool
}

func (n *ImmutableHTMLNode) ImmutableMarkupNode() (core.ImmutableMarkupNode, *sync.Pool) {
	return n, immutableNodePool
}

func (n *ImmutableHTMLNode) IsMarkupElement() bool {
	return n.node.Type == html.ElementNode
}

func (n *ImmutableHTMLNode) MarkupAttributeCount() int {
	return len(n.node.Attr)
}

func (n *ImmutableHTMLNode) MarkupAttributeValue(name string) (value string, present bool) {
	for _, attr := range n.node.Attr {
		if attr.Key == name && attr.Namespace == "" {
			return attr.Val, true
		}
	}
	return "", false
}

func (n *ImmutableHTMLNode) MarkupChild(childIndex int) core.ImmutableMarkupNode {
	currentChild := n.node.FirstChild

	remaining := childIndex

	for remaining > 0 && currentChild != nil {
		currentChild = currentChild.NextSibling
		remaining--
	}

	if currentChild == nil {
		return nil
	}

	immutableChildNode := immutableNodePool.Get().(*ImmutableHTMLNode)
	immutableChildNode.node = currentChild
	return immutableChildNode
}

func (n *ImmutableHTMLNode) MarkupChildNodeCount() int {
	count := 0
	child := n.node.FirstChild
	for child != nil {
		count++
		child = child.NextSibling
	}
	return count
}

func (n *ImmutableHTMLNode) MarkupTagName() (string, bool) {
	if n.node.Type == html.ElementNode {
		return n.node.Data, true
	}
	return "", false
}

func (n *ImmutableHTMLNode) MarkupText() (string, bool) {
	if n.node.Type == html.TextNode {
		return n.node.Data, true
	}
	return "", false
}

func (n *ImmutableHTMLNode) ImmutableMarkupNode_() {}
