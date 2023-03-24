package internal

import (
	symbolic "github.com/inox-project/inox/internal/core/symbolic"
)

var _ = []symbolic.Iterable{&Set{}}

type Set struct {
	symbolic.UnassignablePropsMixin
	_ int
}

func (*Set) Test(v symbolic.SymbolicValue) bool {
	_, ok := v.(*Set)
	return ok
}

func (*Set) Clone(clones map[uintptr]symbolic.SymbolicValue) symbolic.SymbolicValue {
	return &Set{}
}

func (s *Set) GetGoMethod(name string) (*symbolic.GoFunction, bool) {
	switch name {
	case "add":
		return symbolic.WrapGoMethod(s.Add), true
	case "remove":
		return symbolic.WrapGoMethod(s.Remove), true
	}
	return &symbolic.GoFunction{}, false
}

func (s *Set) Prop(name string) symbolic.SymbolicValue {
	return symbolic.GetGoMethodOrPanic(name, s)
}

func (*Set) PropertyNames() []string {
	return []string{"add", "remove"}
}

func (*Set) Add(ctx *symbolic.Context, v symbolic.SymbolicValue) {

}

func (*Set) Remove(ctx *symbolic.Context, v symbolic.SymbolicValue) {

}

func (*Set) Widen() (symbolic.SymbolicValue, bool) {
	return nil, false
}

func (*Set) IsWidenable() bool {
	return false
}

func (*Set) String() string {
	return "set"
}

func (*Set) IteratorElementKey() symbolic.SymbolicValue {
	return &symbolic.Any{}
}

func (*Set) IteratorElementValue() symbolic.SymbolicValue {
	return &symbolic.Any{}
}

func (*Set) WidestOfType() symbolic.SymbolicValue {
	return &Set{}
}
