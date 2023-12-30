package containers

import (
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/prettyprint"
	pprint "github.com/inoxlang/inox/internal/prettyprint"
)

var _ = []symbolic.Iterable{(*Ranking)(nil), (*Rank)(nil)}

type Ranking struct {
	symbolic.UnassignablePropsMixin
	_ int
}

func (r *Ranking) Test(v symbolic.Value, state symbolic.RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	_, ok := v.(*Ranking)
	return ok
}

func (r *Ranking) GetGoMethod(name string) (*symbolic.GoFunction, bool) {
	switch name {
	case "add":
		return symbolic.WrapGoMethod(r.Add), true
	case "remove":
		return symbolic.WrapGoMethod(r.Remove), true
	}
	return nil, false
}

func (r *Ranking) Prop(name string) symbolic.Value {
	return symbolic.GetGoMethodOrPanic(name, r)
}

func (*Ranking) PropertyNames() []string {
	return []string{"add", "remove"}
}

func (f *Ranking) Add(ctx *symbolic.Context, v symbolic.Serializable, score *symbolic.Float) {

}

func (f *Ranking) Remove(ctx *symbolic.Context, v symbolic.Serializable) {

}

func (r *Ranking) PrettyPrint(w prettyprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("ranking")
}

func (r *Ranking) IteratorElementKey() symbolic.Value {
	return symbolic.ANY
}

func (r *Ranking) IteratorElementValue() symbolic.Value {
	return symbolic.ANY
}

func (r *Ranking) WidestOfType() symbolic.Value {
	return &Ranking{}
}

type Rank struct {
	symbolic.UnassignablePropsMixin
	_ int
}

func (r *Rank) Test(v symbolic.Value, state symbolic.RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	_, ok := v.(*Rank)
	return ok
}

func (r *Rank) GetGoMethod(name string) (*symbolic.GoFunction, bool) {
	return nil, false
}

func (r *Rank) Prop(name string) symbolic.Value {
	switch name {
	case "values":
		return symbolic.NewListOf(symbolic.ANY_SERIALIZABLE)
	}
	return symbolic.GetGoMethodOrPanic(name, r)
}

func (*Rank) PropertyNames() []string {
	return []string{"values"}
}

func (r *Rank) PrettyPrint(w prettyprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("rank")
}

func (r *Rank) IteratorElementKey() symbolic.Value {
	return symbolic.ANY
}

func (r *Rank) IteratorElementValue() symbolic.Value {
	return symbolic.ANY
}

func (r *Rank) WidestOfType() symbolic.Value {
	return &Rank{}
}
