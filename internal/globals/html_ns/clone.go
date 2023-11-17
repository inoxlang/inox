package html_ns

import (
	"reflect"
	"slices"

	"github.com/inoxlang/inox/internal/core"
	"golang.org/x/net/html"
)

var (
	_ core.Clonable = (*HTMLNode)(nil)
)

func (n *HTMLNode) Clone(originState *core.GlobalState, sharableValues *[]core.PotentiallySharable, clones map[uintptr]core.Clonable, depth int) (core.Value, error) {
	if depth > core.MAX_CLONING_DEPTH {
		return nil, core.ErrMaximumPseudoCloningDepthReached
	}

	ptr := reflect.ValueOf(n).Pointer()

	if obj, ok := clones[ptr]; ok {
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

	clone.Attr = slices.Clone(n.Attr)

	clone.Parent = cloneHtmlNode(n.Parent, clones)
	clone.PrevSibling = cloneHtmlNode(n.PrevSibling, clones)
	clone.NextSibling = cloneHtmlNode(n.NextSibling, clones)
	clone.FirstChild = cloneHtmlNode(n.FirstChild, clones)
	clone.LastChild = cloneHtmlNode(n.LastChild, clones)

	return clone
}
