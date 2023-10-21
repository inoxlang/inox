package symbolic

import (
	pprint "github.com/inoxlang/inox/internal/pretty_print"
)

// A RandomnessSource represents a symbolic RandomnessSource.
type RandomnessSource struct {
	UnassignablePropsMixin
	_ int
}

func (r *RandomnessSource) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	switch v.(type) {
	case *RandomnessSource:
		return true
	default:
		return false
	}
}

func (r *RandomnessSource) Start(cr *Context) *Error {
	return nil
}

func (r *RandomnessSource) Commit(cr *Context) *Error {
	return nil
}

func (r *RandomnessSource) Rollback(cr *Context) *Error {
	return nil
}

func (r *RandomnessSource) Prop(name string) Value {
	method, ok := r.GetGoMethod(name)
	if !ok {
		panic(FormatErrPropertyDoesNotExist(name, r))
	}
	return method
}

func (r *RandomnessSource) PropertyNames() []string {
	return nil
}

func (r *RandomnessSource) GetGoMethod(name string) (*GoFunction, bool) {
	return nil, false
}

func (r *RandomnessSource) PrettyPrint(w PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("random-source")
	return
}

func (r *RandomnessSource) WidestOfType() Value {
	return &RandomnessSource{}
}
