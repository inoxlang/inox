package internal

import (
	core "github.com/inox-project/inox/internal/core"
)

var (
	_ = []core.Pattern{&NodePattern{}}
)

type NodePattern struct {
	core.NoReprMixin
	core.NotCallablePatternMixin
	core.NotClonableMixin
	modelPattern core.Pattern
}

func (p *NodePattern) Test(ctx *core.Context, v core.Value) bool {
	node, ok := v.(*Node)
	if !ok {
		return false
	}

	return p.modelPattern.Test(ctx, node.model)
}

func (p *NodePattern) Iterator(ctx *core.Context, config core.IteratorConfiguration) core.Iterator {
	return core.NewEmptyPatternIterator()
}

func (p *NodePattern) Random(ctx *core.Context, options ...core.Option) core.Value {
	panic(core.ErrNotImplementedYet)
}

func (patt *NodePattern) StringPattern() (core.StringPattern, bool) {
	return nil, false
}
