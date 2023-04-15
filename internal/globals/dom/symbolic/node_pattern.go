package internal

import (
	symbolic "github.com/inoxlang/inox/internal/core/symbolic"
)

var (
	_ = []symbolic.Pattern{&NodePattern{}}
)

type NodePattern struct {
	symbolic.NotCallablePatternMixin
	modelPattern symbolic.Pattern
}

func NewDomNodePattern(modelPattern symbolic.Pattern) *NodePattern {
	return &NodePattern{modelPattern: modelPattern}
}

func (n *NodePattern) Test(v symbolic.SymbolicValue) bool {
	otherPatt, ok := v.(*NodePattern)
	if !ok {
		return false
	}

	return n.modelPattern.Test(otherPatt.modelPattern)
}

func (n *NodePattern) TestValue(v symbolic.SymbolicValue) bool {
	node, ok := v.(*Node)
	if !ok {
		return false
	}

	return n.modelPattern.TestValue(node.model)
}

func (*NodePattern) HasUnderylingPattern() bool {
	return true
}

func (*NodePattern) Widen() (symbolic.SymbolicValue, bool) {
	return nil, false
}

func (*NodePattern) IsWidenable() bool {
	return false
}

func (p *NodePattern) IteratorElementKey() symbolic.SymbolicValue {
	return &symbolic.Int{}
}

func (p *NodePattern) IteratorElementValue() symbolic.SymbolicValue {
	return &Node{model: p.modelPattern.SymbolicValue()}
}

func (p *NodePattern) SymbolicValue() symbolic.SymbolicValue {
	return &Node{model: p.modelPattern.SymbolicValue()}
}

func (p *NodePattern) StringPattern() (symbolic.StringPatternElement, bool) {
	return nil, false
}

func (r *NodePattern) String() string {
	return "%dom-node-pattern"
}

func (r *NodePattern) WidestOfType() symbolic.SymbolicValue {
	return &NodePattern{}
}
