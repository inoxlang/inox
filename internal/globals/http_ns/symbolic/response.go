package http_ns

import (
	"github.com/inoxlang/inox/internal/core/symbolic"
	pprint "github.com/inoxlang/inox/internal/pretty_print"
)

var (
	ANY_RESP = &HttpResponse{}

	HTTP_RESPONSE_PROPNAMES = []string{"body", "status", "statusCode", "cookies"}
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
	case "statusCode":
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

func (r *HttpResponse) PrettyPrint(w symbolic.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("http-response")
}

func (r *HttpResponse) WidestOfType() symbolic.Value {
	return &HttpResponse{}
}
