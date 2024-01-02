package http_ns

import (
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/prettyprint"
	pprint "github.com/inoxlang/inox/internal/prettyprint"
)

var (
	ANY_RESULT = &HttpResult{}
)

type HttpResult struct {
	symbolic.UnassignablePropsMixin
	_ int
}

func (r *HttpResult) Test(v symbolic.Value, state symbolic.RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	_, ok := v.(*HttpResult)
	return ok
}

func (r *HttpResult) PrettyPrint(w prettyprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("http-result")
}

func (r *HttpResult) WidestOfType() symbolic.Value {
	return &HttpResult{}
}
