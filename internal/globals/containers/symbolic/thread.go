package containers

import (
	"github.com/inoxlang/inox/internal/core/symbolic"
	pprint "github.com/inoxlang/inox/internal/pretty_print"
)

var _ = []symbolic.Iterable{(*Thread)(nil)}

type Thread struct {
	symbolic.UnassignablePropsMixin
	_ int
}

func (*Thread) Test(v symbolic.Value, state symbolic.RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	_, ok := v.(*Thread)
	return ok
}

func (t *Thread) GetGoMethod(name string) (*symbolic.GoFunction, bool) {
	switch name {
	case "push":
		return symbolic.WrapGoMethod(t.Push), true
	}
	return nil, false
}

func (t *Thread) Prop(name string) symbolic.Value {
	return symbolic.GetGoMethodOrPanic(name, t)
}

func (*Thread) PropertyNames() []string {
	return []string{"push"}
}

func (*Thread) Push(ctx *symbolic.Context, elems ...symbolic.Value) {

}

func (*Thread) PrettyPrint(w symbolic.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("thread")
}

func (t *Thread) IteratorElementKey() symbolic.Value {
	return &symbolic.Any{}
}

func (*Thread) IteratorElementValue() symbolic.Value {
	return &symbolic.Any{}
}

func (*Thread) WidestOfType() symbolic.Value {
	return &Thread{}
}
