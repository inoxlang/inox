package symbolic

import (
	pprint "github.com/inoxlang/inox/internal/pretty_print"
)

var (
	ANY_MUTATION = &Mutation{}
)

// An Mutation represents a symbolic Mutation.
type Mutation struct {
	_ int
}

func (r *Mutation) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	_, ok := v.(Iterable)

	return ok
}

func (r *Mutation) PrettyPrint(w PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("mutation")
	return
}

func (r *Mutation) WidestOfType() Value {
	return ANY_MUTATION
}
