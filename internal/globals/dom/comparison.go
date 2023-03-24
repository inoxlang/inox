package internal

import core "github.com/inox-project/inox/internal/core"

func (n *Node) Equal(ctx *core.Context, other core.Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherNode, ok := other.(*Node)
	if !ok {
		return false
	}
	return n == otherNode
}

func (p *NodePattern) Equal(ctx *core.Context, other core.Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherPatt, ok := other.(*NodePattern)
	if !ok {
		return false
	}
	return p == otherPatt
}

func (evs *DomEventSource) Equal(ctx *core.Context, other core.Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherSource, ok := other.(*DomEventSource)
	if !ok {
		return false
	}
	return evs == otherSource
}

func (v *View) Equal(ctx *core.Context, other core.Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherView, ok := other.(*View)
	if !ok {
		return false
	}
	return v == otherView
}

func (c *ContentSecurityPolicy) Equal(ctx *core.Context, other core.Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherCSP, ok := other.(*ContentSecurityPolicy)
	if !ok {
		return false
	}
	if len(c.directives) != len(otherCSP.directives) {
		return false
	}
	for name, directive := range c.directives {
		otherDirective, ok := otherCSP.directives[name]
		if !ok || len(directive.values) != len(otherDirective.values) {
			return false
		}
		for i, val := range directive.values {
			if otherDirective.values[i].raw != val.raw {
				return false
			}
		}
	}
	return true
}
