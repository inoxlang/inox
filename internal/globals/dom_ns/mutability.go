package dom_ns

func (n *Node) IsMutable() bool {
	return true
}

func (p *NodePattern) IsMutable() bool {
	return true
}

func (evs *DomEventSource) IsMutable() bool {
	return true
}

func (v *View) IsMutable() bool {
	return true
}

func (*ContentSecurityPolicy) IsMutable() bool {
	return false
}
