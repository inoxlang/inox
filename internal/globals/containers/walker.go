package internal

import core "github.com/inox-project/inox/internal/core"

var (
	_ = []core.Walkable{(*Graph)(nil)}
)

type GraphWalker struct {
	core.NoReprMixin
	hasNext func(*GraphWalker, *core.Context) bool
	next    func(*GraphWalker, *core.Context) bool
	prune   func(*GraphWalker, *core.Context)
	key     func(*GraphWalker, *core.Context) core.Value
	value   func(*GraphWalker, *core.Context) core.Value
}

func (wk *GraphWalker) HasNext(ctx *core.Context) bool {
	return wk.hasNext(wk, ctx)
}

func (wk *GraphWalker) Next(ctx *core.Context) bool {
	if !wk.HasNext(ctx) {
		return false
	}

	return wk.next(wk, ctx)
}

func (wk *GraphWalker) Prune(ctx *core.Context) {
	wk.prune(wk, ctx)
}

func (wk *GraphWalker) Key(ctx *core.Context) core.Value {
	return wk.key(wk, ctx)
}

func (wk *GraphWalker) Value(ctx *core.Context) core.Value {
	return wk.value(wk, ctx)
}

func (wk *GraphWalker) NodeMeta(*core.Context) core.WalkableNodeMeta {
	panic(core.ErrNotImplementedYet)
}

func newEmptyGraphWalker() *GraphWalker {
	return &GraphWalker{
		hasNext: func(gw *GraphWalker, ctx *core.Context) bool {
			return false
		},
		next: func(gw *GraphWalker, ctx *core.Context) bool {
			return false
		},
	}
}
