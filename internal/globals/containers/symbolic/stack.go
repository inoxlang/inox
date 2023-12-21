package containers

import (
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/prettyprint"
	pprint "github.com/inoxlang/inox/internal/prettyprint"
)

var _ = []symbolic.Iterable{(*Stack)(nil)}

type Stack struct {
	symbolic.UnassignablePropsMixin
	_ int
}

func (*Stack) Test(v symbolic.Value, state symbolic.RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	_, ok := v.(*Stack)
	return ok
}

func (s *Stack) GetGoMethod(name string) (*symbolic.GoFunction, bool) {
	switch name {
	case "push":
		return symbolic.WrapGoMethod(s.Push), true
	case "pop":
		return symbolic.WrapGoMethod(s.Pop), true
	case "peek":
		return symbolic.WrapGoMethod(s.Peek), true
	}
	return nil, false
}

func (s *Stack) Prop(name string) symbolic.Value {
	return symbolic.GetGoMethodOrPanic(name, s)
}

func (*Stack) PropertyNames() []string {
	return []string{"push", "pop", "peek"}
}

func (*Stack) Push(ctx *symbolic.Context, elems ...symbolic.Value) {

}

func (*Stack) Pop(ctx *symbolic.Context) {

}

func (*Stack) Peek(ctx *symbolic.Context) symbolic.Value {
	return &symbolic.Any{}
}

func (*Stack) PrettyPrint(w prettyprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("set")
}

func (*Stack) IteratorElementKey() symbolic.Value {
	return &symbolic.Any{}
}

func (*Stack) IteratorElementValue() symbolic.Value {
	return &symbolic.Any{}
}

func (*Stack) WidestOfType() symbolic.Value {
	return &Stack{}
}
