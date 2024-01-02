package treecoll

import (
	"errors"

	"github.com/inoxlang/inox/internal/core"
)

type TreeNodePattern struct {
	valuePattern core.Pattern
	core.CallBasedPatternReprMixin

	core.NotCallablePatternMixin
}

func (patt *TreeNodePattern) Test(ctx *core.Context, v core.Value) bool {
	node, ok := v.(*TreeNode)
	if !ok {
		return false
	}

	return patt.valuePattern.Test(ctx, node.data)
}

func (patt *TreeNodePattern) Random(ctx *core.Context, options ...core.Option) core.Value {
	panic(errors.New("cannot created random tree node"))
}

func (patt *TreeNodePattern) Iterator(ctx *core.Context, config core.IteratorConfiguration) core.Iterator {
	return core.NewEmptyPatternIterator()
}

func (patt *TreeNodePattern) StringPattern() (core.StringPattern, bool) {
	return nil, false
}

func (p *TreeNodePattern) IsMutable() bool {
	return false
}

func (p *TreeNodePattern) Equal(ctx *core.Context, other core.Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherPattern, ok := other.(*TreeNodePattern)
	if !ok {
		return false
	}

	return p.valuePattern.Equal(ctx, otherPattern.valuePattern, map[uintptr]uintptr{}, 0)
}
