package containers

import (
	"github.com/inoxlang/inox/internal/core/symbolic"
	pprint "github.com/inoxlang/inox/internal/prettyprint"
)

var (
	THREAD_PROPNAMES            = []string{"add"}
	THREAD_ADD_METHOD_ARG_NAMES = []string{"message"}

	_ = []symbolic.Iterable{(*Thread)(nil)}
)

type Thread struct {
	elementPattern symbolic.Pattern
	element        symbolic.Value

	addMethodParamsCache *[]symbolic.Value
}

func newThread(elementPattern symbolic.Pattern) *Thread {
	t := &Thread{
		elementPattern: elementPattern,
		element:        elementPattern.SymbolicValue(),
	}
	t.addMethodParamsCache = &[]symbolic.Value{t.element}
	return t
}

func (t *Thread) Test(v symbolic.Value, state symbolic.RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	otherThread, ok := v.(*Thread)
	return ok && t.elementPattern.Test(otherThread.elementPattern, symbolic.RecTestCallState{})
}

func (t *Thread) GetGoMethod(name string) (*symbolic.GoFunction, bool) {
	switch name {
	case "add":
		return symbolic.WrapGoMethod(t.Add), true
	}
	return nil, false
}

func (t *Thread) Prop(name string) symbolic.Value {
	return symbolic.GetGoMethodOrPanic(name, t)
}

func (*Thread) PropertyNames() []string {
	return THREAD_PROPNAMES
}

func (t *Thread) Add(ctx *symbolic.Context, elem *symbolic.Object) {
	ctx.SetSymbolicGoFunctionParameters(t.addMethodParamsCache, THREAD_ADD_METHOD_ARG_NAMES)
}

func (*Thread) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("thread")
}

func (t *Thread) IteratorElementKey() symbolic.Value {
	return symbolic.ANY
}

func (*Thread) IteratorElementValue() symbolic.Value {
	return symbolic.ANY
}

func (*Thread) WidestOfType() symbolic.Value {
	return &Thread{}
}
