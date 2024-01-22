package containers

func (s *Set) IsMutable() bool {
	return true
}

func (p *SetPattern) IsMutable() bool {
	return false
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

func (p *MapPattern) IsMutable() bool {
	return false
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

func (t *Tree) IsMutable() bool {
	return true
}

func (n TreeNode) IsMutable() bool {
	return true
}

func (p *TreeNodePattern) IsMutable() bool {
	return false
}
