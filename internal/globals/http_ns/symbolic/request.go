package http_ns

import (
	"bufio"

	"github.com/inoxlang/inox/internal/core/symbolic"
	pprint "github.com/inoxlang/inox/internal/pretty_print"

	"github.com/inoxlang/inox/internal/utils"
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

func (r *HttpRequest) Test(v symbolic.SymbolicValue, state symbolic.RecTestCallState) bool {
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

func (req *HttpRequest) Prop(name string) symbolic.SymbolicValue {
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

func (r *HttpRequest) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%http.req")))
}

func (r *HttpRequest) WidestOfType() symbolic.SymbolicValue {
	return &HttpRequest{}
}
