package internal

import (
	symbolic "github.com/inox-project/inox/internal/core/symbolic"
)

type View struct {
	model symbolic.SymbolicValue
}

func NewDomView(model symbolic.SymbolicValue) *View {
	return &View{model: model}
}

func (n *View) Test(v symbolic.SymbolicValue) bool {
	otherView, ok := v.(*View)
	if !ok {
		return false
	}

	return n.model.Test(otherView.model)
}

func (r *View) Widen() (symbolic.SymbolicValue, bool) {
	return nil, false
}

func (r *View) IsWidenable() bool {
	return false
}

func (r *View) String() string {
	return "%dom-view"
}

func (r *View) WidestOfType() symbolic.SymbolicValue {
	return &View{}
}
