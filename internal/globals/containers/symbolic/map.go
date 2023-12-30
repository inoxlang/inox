package containers

import (
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/prettyprint"
	pprint "github.com/inoxlang/inox/internal/prettyprint"
)

var _ = []symbolic.Iterable{(*Map)(nil)}

type Map struct {
	symbolic.UnassignablePropsMixin
	_ int
}

func (*Map) Test(v symbolic.Value, state symbolic.RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	_, ok := v.(*Map)
	return ok
}

func (m *Map) GetGoMethod(name string) (*symbolic.GoFunction, bool) {
	switch name {
	case "insert":
		return symbolic.WrapGoMethod(m.Insert), true
	case "update":
		return symbolic.WrapGoMethod(m.Update), true
	case "remove":
		return symbolic.WrapGoMethod(m.Remove), true
	case "get":
		return symbolic.WrapGoMethod(m.Get), true
	}
	return nil, false
}

func (m *Map) Prop(name string) symbolic.Value {
	return symbolic.GetGoMethodOrPanic(name, m)
}

func (*Map) PropertyNames() []string {
	return []string{"insert", "update", "remove", "get"}
}

func (*Map) Insert(ctx *symbolic.Context, k, v symbolic.Value) {

}

func (*Map) Update(ctx *symbolic.Context, k, v symbolic.Value) {

}

func (*Map) Remove(ctx *symbolic.Context, k symbolic.Value) {

}

func (*Map) Get(ctx *symbolic.Context, k symbolic.Value) symbolic.Value {
	return symbolic.ANY
}

func (*Map) PrettyPrint(w prettyprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("map")
}

func (m *Map) IteratorElementKey() symbolic.Value {
	return symbolic.ANY
}

func (*Map) IteratorElementValue() symbolic.Value {
	return symbolic.ANY
}

func (*Map) WidestOfType() symbolic.Value {
	return &Map{}
}
