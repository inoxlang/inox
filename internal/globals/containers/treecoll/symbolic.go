package treecoll

import (
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/symbolic"
	coll_symbolic "github.com/inoxlang/inox/internal/globals/containers/symbolic"
	"github.com/inoxlang/inox/internal/utils"
)

func (it *TreeIterator) ToSymbolicValue(ctx *core.Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return &symbolic.Iterator{}, nil
}

func (t *Tree) ToSymbolicValue(ctx *core.Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return coll_symbolic.NewTree(t.IsShared()), nil
}

func (n *TreeNode) ToSymbolicValue(ctx *core.Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return coll_symbolic.NewTreeNode(utils.Must(n.tree.ToSymbolicValue(ctx, encountered)).(*coll_symbolic.Tree)), nil
}

func (p *TreeNodePattern) ToSymbolicValue(ctx *core.Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	valuePatt, err := p.valuePattern.ToSymbolicValue(ctx, encountered)
	if err != nil {
		return nil, err
	}
	return coll_symbolic.NewTreeNodePattern(valuePatt.(symbolic.Pattern))
}
