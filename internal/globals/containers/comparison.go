package internal

import (
	core "github.com/inoxlang/inox/internal/core"
)

func (s *Set) Equal(ctx *core.Context, other core.Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherSet, ok := other.(*Set)
	return ok && s == otherSet
}

func (s *Stack) Equal(ctx *core.Context, other core.Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherStack, ok := other.(*Stack)
	return ok && s == otherStack
}

func (q *Queue) Equal(ctx *core.Context, other core.Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherQueue, ok := other.(*Queue)
	return ok && q == otherQueue
}

func (t *Thread) Equal(ctx *core.Context, other core.Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherThread, ok := other.(*Thread)
	return ok && t == otherThread
}

func (m *Map) Equal(ctx *core.Context, other core.Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherMap, ok := other.(*Map)
	return ok && m == otherMap
}

func (g *Graph) Equal(ctx *core.Context, other core.Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherGraph, ok := other.(*Graph)
	return ok && g == otherGraph
}

func (n GraphNode) Equal(ctx *core.Context, other core.Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherNode, ok := other.(GraphNode)
	return ok && n.node_.ID() == otherNode.node_.ID()
}

func (r *Ranking) Equal(ctx *core.Context, other core.Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherRanking, ok := other.(*Ranking)
	return ok && r == otherRanking
}

func (r *Rank) Equal(ctx *core.Context, other core.Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherRank, ok := other.(*Rank)
	return ok && r == otherRank
}

func (it *CollectionIterator) Equal(ctx *core.Context, other core.Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherIt, ok := other.(*CollectionIterator)
	return ok && it == otherIt
}

func (wk *GraphWalker) Equal(ctx *core.Context, other core.Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherWk, ok := other.(*GraphWalker)
	return ok && wk == otherWk
}

func (it *TreeIterator) Equal(ctx *core.Context, other core.Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherIt, ok := other.(*TreeIterator)
	if !ok {
		return false
	}
	return otherIt == it
}

func (t *Tree) Equal(ctx *core.Context, other core.Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherTree, ok := other.(*Tree)
	return ok && t == otherTree
}

func (p *TreeNodePattern) Equal(ctx *core.Context, other core.Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherPattern, ok := other.(*TreeNodePattern)
	if !ok {
		return false
	}

	return p.valuePattern.Equal(ctx, otherPattern.valuePattern, map[uintptr]uintptr{}, 0)
}

func (n *TreeNode) Equal(ctx *core.Context, other core.Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherNode, ok := other.(*TreeNode)
	return ok && n == otherNode
}
