package containers

import (
	"maps"

	core "github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/utils"
)

type CollectionIterator struct {
	hasNext func(*CollectionIterator, *core.Context) bool
	next    func(*CollectionIterator, *core.Context) bool
	key     func(*CollectionIterator, *core.Context) core.Value
	value   func(*CollectionIterator, *core.Context) core.Value

	_key   core.Value
	_value core.Value
}

func (it *CollectionIterator) HasNext(ctx *core.Context) bool {
	return it.hasNext(it, ctx)
}

func (it *CollectionIterator) Next(ctx *core.Context) bool {
	if !it.HasNext(ctx) {
		return false
	}

	return it.next(it, ctx)
}

func (it *CollectionIterator) Key(ctx *core.Context) core.Value {
	return it.key(it, ctx)
}

func (it *CollectionIterator) Value(ctx *core.Context) core.Value {
	return it.value(it, ctx)
}

func (it *CollectionIterator) Iterator(ctx *core.Context, config core.IteratorConfiguration) core.Iterator {
	return it
}

func (g *Graph) Iterator(ctx *core.Context, config core.IteratorConfiguration) core.Iterator {
	it := g.graph.Nodes()
	var next bool

	i := -1

	return config.CreateIterator(&CollectionIterator{
		hasNext: func(ci *CollectionIterator, ctx *core.Context) bool {
			if !next {
				if !it.Next() {
					return false
				}
				next = true
			}
			return true
		},
		next: func(ci *CollectionIterator, ctx *core.Context) bool {
			next = false
			i++
			return true
		},
		key: func(ci *CollectionIterator, ctx *core.Context) core.Value {
			return core.Int(i)
		},
		value: func(ci *CollectionIterator, ctx *core.Context) core.Value {
			node := GraphNode{node_: it.Node(), graph: g}
			return node
		},
	})
}

func (s *Queue) Iterator(ctx *core.Context, config core.IteratorConfiguration) core.Iterator {
	it := s.elements.Iterator()
	var next core.Value

	return config.CreateIterator(&CollectionIterator{
		hasNext: func(ci *CollectionIterator, ctx *core.Context) bool {
			if next == nil {
				if !it.Next() {
					return false
				}
				next = it.Value().(core.Value)
			}
			return true
		},
		next: func(ci *CollectionIterator, ctx *core.Context) bool {
			next = nil
			return true
		},
		key: func(ci *CollectionIterator, ctx *core.Context) core.Value {
			return core.Int(it.Index())
		},
		value: func(ci *CollectionIterator, ctx *core.Context) core.Value {
			return it.Value().(core.Value)
		},
	})
}

func (r *Ranking) Iterator(ctx *core.Context, config core.IteratorConfiguration) core.Iterator {
	rank := -1

	return config.CreateIterator(&CollectionIterator{
		hasNext: func(ci *CollectionIterator, ctx *core.Context) bool {
			return rank < len(r.rankItems)-1
		},
		next: func(ci *CollectionIterator, ctx *core.Context) bool {
			rank++
			return true
		},
		key: func(ci *CollectionIterator, ctx *core.Context) core.Value {
			return core.Int(rank)
		},
		value: func(ci *CollectionIterator, ctx *core.Context) core.Value {
			return &Rank{
				ranking: r,
				rank:    rank,
			}
		},
	})
}

func (s *Map) Iterator(ctx *core.Context, config core.IteratorConfiguration) core.Iterator {
	i := -1
	var ids []core.FastId
	for k := range s.values {
		ids = append(ids, k)
	}

	return config.CreateIterator(&CollectionIterator{
		hasNext: func(ci *CollectionIterator, ctx *core.Context) bool {
			return i < len(ids)-1
		},
		next: func(ci *CollectionIterator, ctx *core.Context) bool {
			i++
			return true
		},
		key: func(ci *CollectionIterator, ctx *core.Context) core.Value {
			return s.keys[ids[i]]
		},
		value: func(ci *CollectionIterator, ctx *core.Context) core.Value {
			return s.values[ids[i]]
		},
	})
}

func (s *Set) Iterator(ctx *core.Context, config core.IteratorConfiguration) core.Iterator {
	i := -1

	closestState := ctx.GetClosestState()
	s.lock.Lock(closestState, s)
	defer s.lock.Unlock(closestState, s)

	elements := maps.Clone(s.elements)

	var keys []string
	for k := range s.elements {
		keys = append(keys, k)
	}

	return config.CreateIterator(&CollectionIterator{
		hasNext: func(ci *CollectionIterator, ctx *core.Context) bool {
			return i < len(keys)-1
		},
		next: func(ci *CollectionIterator, ctx *core.Context) bool {
			i++
			return true
		},
		key: func(ci *CollectionIterator, ctx *core.Context) core.Value {
			return core.Str(keys[i])
		},
		value: func(ci *CollectionIterator, ctx *core.Context) core.Value {
			return elements[keys[i]]
		},
	})
}

func (s *Stack) Iterator(ctx *core.Context, config core.IteratorConfiguration) core.Iterator {
	i := -1

	return config.CreateIterator(&CollectionIterator{
		hasNext: func(ci *CollectionIterator, ctx *core.Context) bool {
			return i < len(s.elements)-1
		},
		next: func(ci *CollectionIterator, ctx *core.Context) bool {
			i++
			return true
		},
		key: func(ci *CollectionIterator, ctx *core.Context) core.Value {
			return core.Int(i)
		},
		value: func(ci *CollectionIterator, ctx *core.Context) core.Value {
			return s.elements[i]
		},
	})
}

func (s *Thread) Iterator(ctx *core.Context, config core.IteratorConfiguration) core.Iterator {
	i := -1

	return config.CreateIterator(&CollectionIterator{
		hasNext: func(ci *CollectionIterator, ctx *core.Context) bool {
			return i < len(s.elements)-1
		},
		next: func(ci *CollectionIterator, ctx *core.Context) bool {
			i++
			return true
		},
		key: func(ci *CollectionIterator, ctx *core.Context) core.Value {
			return core.Int(i)
		},
		value: func(ci *CollectionIterator, ctx *core.Context) core.Value {
			return s.elements[i].value
		},
	})
}

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

func (t *Tree) Iterator(ctx *core.Context, config core.IteratorConfiguration) core.Iterator {
	state := ctx.GetClosestState()
	t.Lock(state)
	defer t.Unlock(state)

	return config.CreateIterator(&TreeIterator{start: t.root, children: utils.CopySlice(t.root.children), i: -1, childIndex: -1})
}

func (node *TreeNode) Iterator(ctx *core.Context, config core.IteratorConfiguration) core.Iterator {
	state := ctx.GetClosestState()
	node.tree.Lock(state)
	defer node.tree.Unlock(state)

	return config.CreateIterator(&TreeIterator{start: node, children: utils.CopySlice(node.children), i: -1, childIndex: -1})
}
