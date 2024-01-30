package http_ns

import (
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/prettyprint"
	pprint "github.com/inoxlang/inox/internal/prettyprint"
)

var (
	HTTP_REQUEST_PROPNAMES = []string{"method", "url", "path", "body" /*"cookies"*/, "headers"}
	ANY_HTTP_REQUEST       = &Request{}
)

type Request struct {
	symbolic.UnassignablePropsMixin
	symbolic.SerializableMixin
	symbolic.PotentiallySharable
}

func (r *Request) Test(v symbolic.Value, state symbolic.RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	_, ok := v.(*Request)
	return ok
}

func (req *Request) IsSharable() (bool, string) {
	return true, ""
}

func (req *Request) Share(originState *symbolic.State) symbolic.PotentiallySharable {
	return req
}

func (req *Request) IsShared() bool {
	return true
}

func (req *Request) GetGoMethod(name string) (*symbolic.GoFunction, bool) {
	return nil, false
}

func (req *Request) Prop(name string) symbolic.Value {
	switch name {
	case "method":
		return symbolic.ANY_STRING
	case "url":
		return &symbolic.URL{}
	case "path":
		return &symbolic.Path{}
	case "body":
		return &symbolic.Reader{}
	case "headers":
		return symbolic.NewAnyKeyRecord(symbolic.NewTupleOf(symbolic.ANY_STRING))
	case "cookies":
		//TODO
		fallthrough
	default:
		return symbolic.GetGoMethodOrPanic(name, req)
	}
}

func (Request) PropertyNames() []string {
	return HTTP_REQUEST_PROPNAMES
}

func (r *Request) PrettyPrint(w prettyprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("http.req")
}

func (r *Request) WidestOfType() symbolic.Value {
	return &Request{}
}
