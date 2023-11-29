package http_ns

import (
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/prettyprint"
	pprint "github.com/inoxlang/inox/internal/prettyprint"
)

var (
	HTTP_REQUEST_PROPNAMES = []string{"method", "url", "path", "body" /*"cookies"*/, "headers"}
	ANY_HTTP_REQUEST       = &HttpRequest{}
)

type HttpRequest struct {
	symbolic.UnassignablePropsMixin
	symbolic.SerializableMixin
	symbolic.PotentiallySharable
}

func (r *HttpRequest) Test(v symbolic.Value, state symbolic.RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	_, ok := v.(*HttpRequest)
	return ok
}

func (req *HttpRequest) IsSharable() (bool, string) {
	return true, ""
}

func (req *HttpRequest) Share(originState *symbolic.State) symbolic.PotentiallySharable {
	return req
}

func (req *HttpRequest) IsShared() bool {
	return true
}

func (req *HttpRequest) GetGoMethod(name string) (*symbolic.GoFunction, bool) {
	return nil, false
}

func (req *HttpRequest) Prop(name string) symbolic.Value {
	switch name {
	case "method":
		return &symbolic.String{}
	case "url":
		return &symbolic.URL{}
	case "path":
		return &symbolic.Path{}
	case "body":
		return &symbolic.Reader{}
	case "headers":
		return symbolic.NewAnyKeyRecord(symbolic.NewTupleOf(&symbolic.String{}))
	case "cookies":
		//TODO
		fallthrough
	default:
		return symbolic.GetGoMethodOrPanic(name, req)
	}
}

func (HttpRequest) PropertyNames() []string {
	return HTTP_REQUEST_PROPNAMES
}

func (r *HttpRequest) PrettyPrint(w prettyprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("http.req")
}

func (r *HttpRequest) WidestOfType() symbolic.Value {
	return &HttpRequest{}
}
