package internal

import (
	symbolic "github.com/inoxlang/inox/internal/core/symbolic"
	_dom_symbolic "github.com/inoxlang/inox/internal/globals/dom/symbolic"
)

func (n *Node) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	model, err := n.model.ToSymbolicValue(wide, encountered)
	if err != nil {
		return nil, err
	}
	return _dom_symbolic.NewDomNode(model), nil
}

func (p *NodePattern) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	patt, err := p.modelPattern.ToSymbolicValue(wide, encountered)
	if err != nil {
		return nil, err
	}
	return _dom_symbolic.NewDomNodePattern(patt.(symbolic.Pattern)), nil
}

func (evs *DomEventSource) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return symbolic.NewEventSource(), nil
}

func (v *View) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	model, err := v.model.ToSymbolicValue(wide, encountered)
	if err != nil {
		return nil, err
	}

	return _dom_symbolic.NewDomView(model), nil
}

func (*ContentSecurityPolicy) ToSymbolicValue(wide bool, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return _dom_symbolic.NewCSP(), nil
}
