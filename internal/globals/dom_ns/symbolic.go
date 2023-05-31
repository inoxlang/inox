package dom_ns

import (
	core "github.com/inoxlang/inox/internal/core"
	symbolic "github.com/inoxlang/inox/internal/core/symbolic"
	_dom_symbolic "github.com/inoxlang/inox/internal/globals/dom_ns/symbolic"
)

func (n *Node) ToSymbolicValue(ctx *core.Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	model, err := n.model.ToSymbolicValue(ctx, encountered)
	if err != nil {
		return nil, err
	}
	return _dom_symbolic.NewDomNode(model), nil
}

func (p *NodePattern) ToSymbolicValue(ctx *core.Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	patt, err := p.modelPattern.ToSymbolicValue(ctx, encountered)
	if err != nil {
		return nil, err
	}
	return _dom_symbolic.NewDomNodePattern(patt.(symbolic.Pattern)), nil
}

func (evs *DomEventSource) ToSymbolicValue(ctx *core.Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return symbolic.NewEventSource(), nil
}

func (v *View) ToSymbolicValue(ctx *core.Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	model, err := v.model.ToSymbolicValue(ctx, encountered)
	if err != nil {
		return nil, err
	}

	return _dom_symbolic.NewDomView(model), nil
}

func (*ContentSecurityPolicy) ToSymbolicValue(ctx *core.Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error) {
	return _dom_symbolic.NewCSP(), nil
}
