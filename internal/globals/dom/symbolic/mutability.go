package internal

func (n *Node) IsMutable() bool {
	return true
}

func (p *NodePattern) IsMutable() bool {
	return false
}

func (p *View) IsMutable() bool {
	return true
}

func (*ContentSecurityPolicy) IsMutable() bool {
	return false
}
