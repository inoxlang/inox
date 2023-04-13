package internal

import symbolic "github.com/inox-project/inox/internal/core/symbolic"

type ObjectInfo struct {
	symbolic.UnassignablePropsMixin
	_ int
}

func (r *ObjectInfo) Test(v symbolic.SymbolicValue) bool {
	_, ok := v.(*ObjectInfo)
	return ok
}

func (r ObjectInfo) Clone(clones map[uintptr]symbolic.SymbolicValue) symbolic.SymbolicValue {
	return &ObjectInfo{}
}

func (resp *ObjectInfo) GetGoMethod(name string) (*symbolic.GoFunction, bool) {
	return &symbolic.GoFunction{}, false
}

func (resp *ObjectInfo) Prop(name string) symbolic.SymbolicValue {
	switch name {
	case "key":
		return &symbolic.String{}
	default:
		return symbolic.GetGoMethodOrPanic(name, resp)
	}
}

func (*ObjectInfo) PropertyNames() []string {
	return []string{"key"}
}

func (r *ObjectInfo) Widen() (symbolic.SymbolicValue, bool) {
	return nil, false
}

func (a *ObjectInfo) IsWidenable() bool {
	return false
}

func (r *ObjectInfo) String() string {
	return "%object-info"
}

func (r *ObjectInfo) WidestOfType() symbolic.SymbolicValue {
	return &ObjectInfo{}
}
