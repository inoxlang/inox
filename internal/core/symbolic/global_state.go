package symbolic

import (
	pprint "github.com/inoxlang/inox/internal/prettyprint"
)

var (
	GLOBAL_STATE_PROPNAMES = []string{"module"}

	ANY_GLOBAL_STATE = &GlobalState{}
)

// A GlobalState represents a symbolic GlobalState.
type GlobalState struct {
	UnassignablePropsMixin
	_ int
}

func (r *GlobalState) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	switch v.(type) {
	case *GlobalState:
		return true
	default:
		return false
	}
}

func (r *GlobalState) WidestOfType() Value {
	return ANY_GLOBAL_STATE
}

func (r *GlobalState) GetGoMethod(name string) (*GoFunction, bool) {
	switch name {
	}
	return nil, false
}

func (r *GlobalState) Prop(name string) Value {
	switch name {
	case "module":
		return ANY_MODULE
	}
	method, ok := r.GetGoMethod(name)
	if !ok {
		panic(FormatErrPropertyDoesNotExist(name, r))
	}
	return method
}

func (*GlobalState) PropertyNames() []string {
	return GLOBAL_STATE_PROPNAMES
}

func (GlobalState *GlobalState) WaitResult(ctx *Context) (Value, *Error) {
	return ANY, nil
}

func (GlobalState *GlobalState) Cancel(*Context) {

}

func (r *GlobalState) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("global-state")
	return
}
