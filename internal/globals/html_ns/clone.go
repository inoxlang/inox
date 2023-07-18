package html_ns

import (
	"reflect"

	core "github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/utils"
	"golang.org/x/net/html"
)

func (n *HTMLNode) Clone(clones map[uintptr]map[int]core.Value, depth int) (core.Value, error) {
	if depth > core.MAX_CLONING_DEPTH {
		return nil, core.ErrMaximumPseudoCloningDepthReached
	}

	ptr := reflect.ValueOf(n).Pointer()

	if obj, ok := clones[ptr][0]; ok {
		return obj, nil
	}

	n.cloneOnWrite = true

	return &HTMLNode{
		node:         n.node,
		cloneOnWrite: true,
	}, nil
}

func cloneHtmlNode(n *html.Node, clones map[*html.Node]*html.Node) *html.Node {
	if n == nil {
		return nil
	}

	if val, ok := clones[n]; ok {
		return val
	}

	clone := &html.Node{}
	*clone = *n
	clones[n] = clone

	clone.Attr = utils.CopySlice(n.Attr)

	clone.Parent = cloneHtmlNode(n.Parent, clones)
	clone.PrevSibling = cloneHtmlNode(n.PrevSibling, clones)
	clone.NextSibling = cloneHtmlNode(n.NextSibling, clones)
	clone.FirstChild = cloneHtmlNode(n.FirstChild, clones)
	clone.LastChild = cloneHtmlNode(n.LastChild, clones)

	return clone
}
