package http_ns

import (
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/prettyprint"
	pprint "github.com/inoxlang/inox/internal/prettyprint"
)

var (
	ANY_RESP = &Response{}

	HTTP_RESPONSE_PROPNAMES = []string{"body", "status", "status-code", "cookies"}
)

type Response struct {
	symbolic.UnassignablePropsMixin
	_ int
}

func (r *Response) Test(v symbolic.Value, state symbolic.RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	_, ok := v.(*Response)
	return ok
}

func (resp *Response) GetGoMethod(name string) (*symbolic.GoFunction, bool) {
	return nil, false
}

func (resp *Response) Prop(name string) symbolic.Value {
	switch name {
	case "body":
		return &symbolic.Reader{}
	case "status":
		return symbolic.ANY_STRING
	case "status-code":
		return ANY_STATUS_CODE
	case "cookies":
		return symbolic.NewListOf(NewCookieObject())
	default:
		return symbolic.GetGoMethodOrPanic(name, resp)
	}
}

func (*Response) PropertyNames() []string {
	return HTTP_RESPONSE_PROPNAMES
}

func (r *Response) PrettyPrint(w prettyprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("http-response")
}

func (r *Response) WidestOfType() symbolic.Value {
	return &Response{}
}
