package http_ns

import (
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/prettyprint"
	pprint "github.com/inoxlang/inox/internal/prettyprint"
)

var (
	ANY_RESULT = &Result{}
)

type Result struct {
	symbolic.UnassignablePropsMixin
	_ int
}

func (r *Result) Test(v symbolic.Value, state symbolic.RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	_, ok := v.(*Result)
	return ok
}

func (r *Result) PrettyPrint(w prettyprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("http-result")
}

func (r *Result) WidestOfType() symbolic.Value {
	return &Result{}
}
