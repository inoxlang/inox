package internal

import (
	"github.com/PuerkitoBio/goquery"
	core "github.com/inox-project/inox/internal/core"
)

func _html_find(ctx *core.Context, selector core.Str, node core.Value) []*HTMLNode {
	doc := goquery.NewDocumentFromNode(node.(*HTMLNode).node)
	nodes := doc.Find(string(selector)).Nodes

	var _nodes []*HTMLNode
	for _, e := range nodes {
		_nodes = append(_nodes, NewHTMLNode(e))
	}

	return _nodes
}
