package internal

import core "github.com/inoxlang/inox/internal/core"

func (n *HTMLNode) Equal(ctx *core.Context, other core.Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherNode, ok := other.(*HTMLNode)
	if !ok {
		return false
	}
	return n.node == otherNode.node
}
