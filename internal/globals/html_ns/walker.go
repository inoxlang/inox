package html_ns

import (
	"errors"

	"golang.org/x/net/html"
)

const MAX_WALKING_DEPTH = 1000

func walkHTMLNode(n *html.Node, fn func(n *html.Node) error, depth int) error {
	if n == nil {
		return nil
	}

	if depth > MAX_WALKING_DEPTH {
		return errors.New("max walking depth reached")
	}

	err := fn(n)
	if err != nil {
		return err
	}

	child := n.FirstChild
	for child != nil {
		walkHTMLNode(child, fn, depth+1)
		child = child.NextSibling
	}

	return nil
}
