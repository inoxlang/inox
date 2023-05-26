package internal

import (
	core "github.com/inoxlang/inox/internal/core"
	symbolic "github.com/inoxlang/inox/internal/core/symbolic"
	coll_symbolic "github.com/inoxlang/inox/internal/globals/containers/symbolic"
	"github.com/inoxlang/inox/internal/utils"
)

func (s *Set) ToSymbolicValue(ctx *core.Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &coll_symbolic.Set{}, nil
}

func (s *Stack) ToSymbolicValue(ctx *core.Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &coll_symbolic.Stack{}, nil
}

func (q *Queue) ToSymbolicValue(ctx *core.Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &coll_symbolic.Queue{}, nil
}

func (t *Thread) ToSymbolicValue(ctx *core.Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &coll_symbolic.Thread{}, nil
}

func (m *Map) ToSymbolicValue(ctx *core.Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &coll_symbolic.Map{}, nil
}

func (g *Graph) ToSymbolicValue(ctx *core.Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &coll_symbolic.Graph{}, nil
}

func (n GraphNode) ToSymbolicValue(ctx *core.Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &coll_symbolic.GraphNode{}, nil
}

func (r *Ranking) ToSymbolicValue(ctx *core.Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &coll_symbolic.Ranking{}, nil
}

func (r *Rank) ToSymbolicValue(ctx *core.Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &coll_symbolic.Rank{}, nil
}

func (it *CollectionIterator) ToSymbolicValue(ctx *core.Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return nil, symbolic.ErrNoSymbolicValue
}

func (it *GraphWalker) ToSymbolicValue(ctx *core.Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return nil, symbolic.ErrNoSymbolicValue
}

func (it *TreeIterator) ToSymbolicValue(ctx *core.Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.Iterator{}, nil
}

func (t *Tree) ToSymbolicValue(ctx *core.Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return coll_symbolic.NewTree(t.IsShared()), nil
}

func (n *TreeNode) ToSymbolicValue(ctx *core.Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return coll_symbolic.NewTreeNode(utils.Must(n.tree.ToSymbolicValue(ctx, encountered)).(*coll_symbolic.Tree)), nil
}

func (p *TreeNodePattern) ToSymbolicValue(ctx *core.Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	valuePatt, err := p.valuePattern.ToSymbolicValue(ctx, encountered)
	if err != nil {
		return nil, err
	}
	return coll_symbolic.NewTreeNodePattern(valuePatt.(symbolic.Pattern))
}
