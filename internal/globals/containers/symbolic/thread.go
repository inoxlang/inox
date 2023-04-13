package internal

import (
	symbolic "github.com/inox-project/inox/internal/core/symbolic"
)

var _ = []symbolic.Iterable{&Thread{}}

type Thread struct {
	symbolic.UnassignablePropsMixin
	_ int
}

func (*Thread) Test(v symbolic.SymbolicValue) bool {
	_, ok := v.(*Thread)
	return ok
}

func (Thread) Clone(clones map[uintptr]symbolic.SymbolicValue) symbolic.SymbolicValue {
	return &Thread{}
}

func (t *Thread) GetGoMethod(name string) (*symbolic.GoFunction, bool) {
	switch name {
	case "push":
		return symbolic.WrapGoMethod(t.Push), true
	}
	return &symbolic.GoFunction{}, false
}

func (t *Thread) Prop(name string) symbolic.SymbolicValue {
	return symbolic.GetGoMethodOrPanic(name, t)
}

func (*Thread) PropertyNames() []string {
	return []string{"push"}
}

func (*Thread) Push(ctx *symbolic.Context, elems ...symbolic.SymbolicValue) {

}

func (*Thread) Widen() (symbolic.SymbolicValue, bool) {
	return nil, false
}

func (a *Thread) IsWidenable() bool {
	return false
}

func (*Thread) String() string {
	return "%thread"
}

func (t *Thread) IteratorElementKey() symbolic.SymbolicValue {
	return &symbolic.Any{}
}

func (*Thread) IteratorElementValue() symbolic.SymbolicValue {
	return &symbolic.Any{}
}

func (*Thread) WidestOfType() symbolic.SymbolicValue {
	return &Thread{}
}
