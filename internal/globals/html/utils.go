package internal

import (
	"github.com/inoxlang/inox/internal/utils"
	"golang.org/x/net/html"
)

func computeApproximativePrintCost(n *html.Node) int64 {
	child := n.FirstChild
	cost := int64(len(n.Data))

	for _, attr := range n.Attr {
		cost += int64(len(attr.Key))
		cost += int64(len(attr.Val))
	}

	for child != nil {
		cost += computeApproximativePrintCost(child)
		child = child.NextSibling
	}

	return cost
}

func computeApproximateRequiredLines(n *html.Node) int64 {
	//TODO: improve

	lineCount := int64(0)
	walkHTMLNode(n, func(n *html.Node) {
		if n.Type == html.TextNode {
			lineCount += 1 //obviously wrong, a text node can have many characters
		} else {
			lineCount += 3
		}
	}, 0)

	return lineCount
}

func computeNodeHeight(root *html.Node) int {
	highestChildHeight := 0

	child := root.FirstChild

	for child != nil {
		highestChildHeight = utils.Max(highestChildHeight, computeNodeHeight(child))
		child = child.NextSibling
	}

	return 1 + highestChildHeight
}

type nodeLevel struct {
	node  *html.Node
	level int
}

func computeNodeWidth(root *html.Node) int {
	if root == nil {
		return 0
	}

	queue := []*nodeLevel{{node: root, level: 0}}
	maxWidth := 0
	levelCount := 0
	currentLevel := 0

	for len(queue) > 0 {
		_node := queue[0]
		queue = queue[1:]

		if _node.level != currentLevel {
			maxWidth = utils.Max(maxWidth, levelCount)
			levelCount = 0
			currentLevel = _node.level
		}
		levelCount++

		for child := _node.node.FirstChild; child != nil; child = child.NextSibling {
			queue = append(queue, &nodeLevel{node: child, level: currentLevel + 1})
		}
	}

	maxWidth = utils.Max(maxWidth, levelCount)
	return maxWidth
}
