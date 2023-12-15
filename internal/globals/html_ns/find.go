package html_ns

import (
	"github.com/PuerkitoBio/goquery"
	"github.com/inoxlang/inox/internal/core"
)

func _html_find(ctx *core.Context, selector core.Str, node *HTMLNode) []*HTMLNode {
	doc := goquery.NewDocumentFromNode(node.node)
	nodes := doc.Find(string(selector)).Nodes

	var _nodes []*HTMLNode
	for _, e := range nodes {
		_nodes = append(_nodes, NewHTMLNode(e))
	}

	return _nodes
}
