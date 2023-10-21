package http_ns

import (
	"bufio"

	"github.com/inoxlang/inox/internal/core/symbolic"
	pprint "github.com/inoxlang/inox/internal/pretty_print"

	"github.com/inoxlang/inox/internal/utils"
)

var (
	_ symbolic.Pattern = (*HttpRequestPattern)(nil)

	ANY_REQUEST_PATTERN = &HttpRequestPattern{}
)

type HttpRequestPattern struct {
	_ int
	symbolic.UnassignablePropsMixin
	symbolic.SerializableMixin
	symbolic.NotCallablePatternMixin
}

func (r *HttpRequestPattern) Test(v symbolic.Value, state symbolic.RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	_, ok := v.(*HttpRequestPattern)
	return ok
}

func (r *HttpRequestPattern) TestValue(v symbolic.Value, state symbolic.RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()
	_, ok := v.(*HttpRequest)
	return ok
}

func (r *HttpRequestPattern) SymbolicValue() symbolic.Value {
	return ANY_HTTP_REQUEST
}

func (r *HttpRequestPattern) HasUnderlyingPattern() bool {
	return true
}

func (r *HttpRequestPattern) IteratorElementKey() symbolic.Value {
	return symbolic.ANY
}

func (r *HttpRequestPattern) IteratorElementValue() symbolic.Value {
	return symbolic.ANY
}

func (r *HttpRequestPattern) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%http.request-pattern")))
	return
}

func (r *HttpRequestPattern) WidestOfType() symbolic.Value {
	return ANY_REQUEST_PATTERN
}

func (r *HttpRequestPattern) StringPattern() (symbolic.StringPattern, bool) {
	return nil, false
}
