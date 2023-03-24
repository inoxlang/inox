package internal

import symbolic "github.com/inox-project/inox/internal/core/symbolic"

type Bucket struct {
	symbolic.UnassignablePropsMixin
	_ int
}

func (r *Bucket) Test(v symbolic.SymbolicValue) bool {
	_, ok := v.(*Bucket)
	return ok
}

func (r Bucket) Clone(clones map[uintptr]symbolic.SymbolicValue) symbolic.SymbolicValue {
	return &Bucket{}
}

func (serv *Bucket) GetGoMethod(name string) (*symbolic.GoFunction, bool) {
	return &symbolic.GoFunction{}, false
}

func (b *Bucket) Prop(name string) symbolic.SymbolicValue {
	return symbolic.GetGoMethodOrPanic(name, b)
}

func (*Bucket) PropertyNames() []string {
	return nil
}

func (r *Bucket) Widen() (symbolic.SymbolicValue, bool) {
	return nil, false
}

func (a *Bucket) IsWidenable() bool {
	return false
}

func (r *Bucket) String() string {
	return "s3-bucket"
}

func (r *Bucket) WidestOfType() symbolic.SymbolicValue {
	return &Bucket{}
}
