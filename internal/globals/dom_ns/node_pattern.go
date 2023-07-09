package dom_ns

import core "github.com/inoxlang/inox/internal/core"

var (
	_ = []core.Pattern{&NodePattern{}}
)

type NodePattern struct {
	modelPattern core.Pattern
	core.CallBasedPatternReprMixin

	core.NotCallablePatternMixin
	core.NotClonableMixin
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
