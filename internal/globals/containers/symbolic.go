package internal

import (
	symbolic "github.com/inox-project/inox/internal/core/symbolic"
	coll_symbolic "github.com/inox-project/inox/internal/globals/containers/symbolic"
	"github.com/inox-project/inox/internal/utils"
)

func (s *Set) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &coll_symbolic.Set{}, nil
}

func (s *Stack) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &coll_symbolic.Stack{}, nil
}

func (q *Queue) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &coll_symbolic.Queue{}, nil
}

func (t *Thread) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &coll_symbolic.Thread{}, nil
}

func (m *Map) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &coll_symbolic.Map{}, nil
}

func (g *Graph) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &coll_symbolic.Graph{}, nil
}

func (n GraphNode) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &coll_symbolic.GraphNode{}, nil
}

func (r *Ranking) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &coll_symbolic.Ranking{}, nil
}

func (r *Rank) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &coll_symbolic.Rank{}, nil
}

func (it *CollectionIterator) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return nil, symbolic.ErrNoSymbolicValue
}

func (it *GraphWalker) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return nil, symbolic.ErrNoSymbolicValue
}

func (it *TreeIterator) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return &symbolic.Iterator{}, nil
}

func (t *Tree) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return coll_symbolic.NewTree(t.IsShared()), nil
}

func (n *TreeNode) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return coll_symbolic.NewTreeNode(utils.Must(n.tree.ToSymbolicValue(wide, encountered)).(*coll_symbolic.Tree)), nil
}

func (p *TreeNodePattern) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	valuePatt, err := p.valuePattern.ToSymbolicValue(wide, encountered)
	if err != nil {
		return nil, err
	}
	return coll_symbolic.NewTreeNodePattern(valuePatt.(symbolic.Pattern))
}
