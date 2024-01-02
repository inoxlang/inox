package treecoll

import (
	"slices"

	"github.com/inoxlang/inox/internal/core"
)

type TreeIterator struct {
	start         *TreeNode
	children      []*TreeNode
	childIndex    int
	i             int
	childIterator *TreeIterator
}

func (it TreeIterator) HasNext(ctx *core.Context) bool {
	return it.i == -1 ||
		(it.children != nil && it.childIndex < len(it.children)-1) ||
		(it.childIterator != nil && it.childIterator.HasNext(ctx))
}

func (it *TreeIterator) Next(ctx *core.Context) bool {
	if !it.HasNext(ctx) {
		return false
	}

	it.i++

	if it.i == 0 {
		it.childIndex++
		if it.children != nil && len(it.children) > 0 {
			it.childIterator = (it.children)[0].Iterator(ctx, core.IteratorConfiguration{}).(*TreeIterator)
		}
	} else if !it.childIterator.Next(ctx) {
		it.childIndex++
		it.childIterator = (it.children)[it.childIndex].Iterator(ctx, core.IteratorConfiguration{}).(*TreeIterator)
		it.childIterator.Next(ctx)
	}

	return true
}

func (it *TreeIterator) Key(ctx *core.Context) core.Value {
	return core.Int(it.i)
}

func (it *TreeIterator) Value(ctx *core.Context) core.Value {
	switch it.i {
	case 0:
		return it.start
	default:
		return it.childIterator.Value(ctx)
	}
}

func (it *TreeIterator) Iterator(ctx *core.Context, config core.IteratorConfiguration) core.Iterator {
	return config.CreateIterator(&TreeIterator{start: it.start, i: -1, childIndex: -1})
}

func (it *TreeIterator) Equal(ctx *core.Context, other core.Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherIt, ok := other.(*TreeIterator)
	if !ok {
		return false
	}
	return otherIt == it
}

// -----------------------------

func (t *Tree) Iterator(ctx *core.Context, config core.IteratorConfiguration) core.Iterator {
	state := ctx.GetClosestState()
	t.Lock(state)
	defer t.Unlock(state)

	return config.CreateIterator(&TreeIterator{start: t.root, children: slices.Clone(t.root.children), i: -1, childIndex: -1})
}

func (node *TreeNode) Iterator(ctx *core.Context, config core.IteratorConfiguration) core.Iterator {
	state := ctx.GetClosestState()
	node.tree.Lock(state)
	defer node.tree.Unlock(state)

	return config.CreateIterator(&TreeIterator{start: node, children: slices.Clone(node.children), i: -1, childIndex: -1})
}
