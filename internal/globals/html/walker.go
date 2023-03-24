package internal

import "golang.org/x/net/html"

const MAX_WALKING_DEPTH = 1000

func walkHTMLNode(n *html.Node, fn func(n *html.Node), depth int) {
	if n == nil || depth > MAX_WALKING_DEPTH {
		return
	}

	fn(n)

	child := n.FirstChild
	for child != nil {
		walkHTMLNode(child, fn, depth+1)
		child = child.NextSibling
	}
}
