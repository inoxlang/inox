package internal

import (
	"reflect"

	"github.com/inoxlang/inox/internal/core"
)

func (s *Set) Clone(clones map[uintptr]map[int]core.Value) (core.Value, error) {
	return nil, core.ErrNotClonable
}

func (s *Stack) Clone(clones map[uintptr]map[int]core.Value) (core.Value, error) {
	return nil, core.ErrNotClonable
}

func (q *Queue) Clone(clones map[uintptr]map[int]core.Value) (core.Value, error) {
	return nil, core.ErrNotClonable
}

func (t *Thread) Clone(clones map[uintptr]map[int]core.Value) (core.Value, error) {
	return nil, core.ErrNotClonable
}

func (m *Map) Clone(clones map[uintptr]map[int]core.Value) (core.Value, error) {
	return nil, core.ErrNotClonable
}

func (g *Graph) Clone(clones map[uintptr]map[int]core.Value) (core.Value, error) {
	return nil, core.ErrNotClonable
}

func (n GraphNode) Clone(clones map[uintptr]map[int]core.Value) (core.Value, error) {
	return nil, core.ErrNotClonable
}

func (r *Ranking) Clone(clones map[uintptr]map[int]core.Value) (core.Value, error) {
	return nil, core.ErrNotClonable
}

func (r *Rank) Clone(clones map[uintptr]map[int]core.Value) (core.Value, error) {
	return nil, core.ErrNotClonable
}

func (it *CollectionIterator) Clone(clones map[uintptr]map[int]core.Value) (core.Value, error) {
	return nil, core.ErrNotClonable
}

func (it *GraphWalker) Clone(clones map[uintptr]map[int]core.Value) (core.Value, error) {
	return nil, core.ErrNotClonable
}

func (t *Tree) Clone(clones map[uintptr]map[int]core.Value) (core.Value, error) {
	return nil, core.ErrNotClonable
}

func (n TreeNode) Clone(clones map[uintptr]map[int]core.Value) (core.Value, error) {
	return nil, core.ErrNotClonable
}

func (p *TreeNodePattern) Clone(clones map[uintptr]map[int]core.Value) (core.Value, error) {
	return nil, core.ErrNotClonable
}

func (it *TreeIterator) Clone(clones map[uintptr]map[int]core.Value) (core.Value, error) {
	return nil, core.ErrNotClonable
}

func (pattern *SetPattern) Clone(clones map[uintptr]map[int]core.Value) (core.Value, error) {
	ptr := reflect.ValueOf(pattern).Pointer()

	if clone, ok := clones[ptr][0]; ok {
		return clone, nil
	}

	clone := &SetPattern{}
	clones[ptr] = map[int]core.Value{0: clone}

	elemenPatternClone, err := pattern.config.Element.Clone(clones)
	if err != nil {
		return nil, err
	}

	clone.config = SetConfig{
		Element:    elemenPatternClone.(core.Pattern),
		Uniqueness: pattern.config.Uniqueness,
	}
	return clone, nil
}
