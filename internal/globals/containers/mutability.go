package internal

func (s *Set) IsMutable() bool {
	return true
}

func (s *Stack) IsMutable() bool {
	return true
}

func (q *Queue) IsMutable() bool {
	return true
}

func (t *Thread) IsMutable() bool {
	return true
}

func (m *Map) IsMutable() bool {
	return true
}

func (g *Graph) IsMutable() bool {
	return true
}

func (n GraphNode) IsMutable() bool {
	return true
}

func (r *Ranking) IsMutable() bool {
	return true
}

func (r *Rank) IsMutable() bool {
	return true
}

func (it *CollectionIterator) IsMutable() bool {
	return true
}

func (wk *GraphWalker) IsMutable() bool {
	return true
}

func (it *TreeIterator) IsMutable() bool {
	return true
}

func (t *Tree) IsMutable() bool {
	return true
}

func (n TreeNode) IsMutable() bool {
	return true
}

func (p *TreeNodePattern) IsMutable() bool {
	return false
}

func (p *SetPattern) IsMutable() bool {
	return false
}
