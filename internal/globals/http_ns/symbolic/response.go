package http_ns

import (
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/prettyprint"
	pprint "github.com/inoxlang/inox/internal/prettyprint"
)

var (
	ANY_RESP = &HttpResponse{}

	HTTP_RESPONSE_PROPNAMES = []string{"body", "status", "status_code", "cookies"}
)

type HttpResponse struct {
	symbolic.UnassignablePropsMixin
	_ int
}

func (r *HttpResponse) Test(v symbolic.Value, state symbolic.RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	_, ok := v.(*HttpResponse)
	return ok
}

func (resp *HttpResponse) GetGoMethod(name string) (*symbolic.GoFunction, bool) {
	return nil, false
}

func (resp *HttpResponse) Prop(name string) symbolic.Value {
	switch name {
	case "body":
		return &symbolic.Reader{}
	case "status":
		return &symbolic.String{}
	case "status_code":
		return &symbolic.Int{}
	case "cookies":
		return symbolic.NewListOf(NewCookieObject())
	default:
		return symbolic.GetGoMethodOrPanic(name, resp)
	}
}

func (*HttpResponse) PropertyNames() []string {
	return HTTP_RESPONSE_PROPNAMES
}

func (r *HttpResponse) PrettyPrint(w prettyprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("http-response")
}

func (r *HttpResponse) WidestOfType() symbolic.Value {
	return &HttpResponse{}
}
