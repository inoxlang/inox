package http_ns

import (
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/prettyprint"
	pprint "github.com/inoxlang/inox/internal/prettyprint"
)

var (
	_ symbolic.Pattern = (*RequestPattern)(nil)

	ANY_REQUEST_PATTERN = &RequestPattern{}
)

type RequestPattern struct {
	_ int
	symbolic.UnassignablePropsMixin
	symbolic.SerializableMixin
	symbolic.NotCallablePatternMixin
}

func (r *RequestPattern) Test(v symbolic.Value, state symbolic.RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	_, ok := v.(*RequestPattern)
	return ok
}

func (r *RequestPattern) TestValue(v symbolic.Value, state symbolic.RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()
	_, ok := v.(*Request)
	return ok
}

func (r *RequestPattern) SymbolicValue() symbolic.Value {
	return ANY_HTTP_REQUEST
}

func (r *RequestPattern) HasUnderlyingPattern() bool {
	return true
}

func (r *RequestPattern) IteratorElementKey() symbolic.Value {
	return symbolic.ANY
}

func (r *RequestPattern) IteratorElementValue() symbolic.Value {
	return symbolic.ANY
}

func (r *RequestPattern) PrettyPrint(w prettyprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("http.request-pattern")
}

func (r *RequestPattern) WidestOfType() symbolic.Value {
	return ANY_REQUEST_PATTERN
}

func (r *RequestPattern) StringPattern() (symbolic.StringPattern, bool) {
	return nil, false
}
