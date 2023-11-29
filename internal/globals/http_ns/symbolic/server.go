package http_ns

import (
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/prettyprint"
	pprint "github.com/inoxlang/inox/internal/prettyprint"
)

var (
	HTTP_SERVER_PROPNAMES = []string{"wait_closed", "close"}
)

type HttpServer struct {
	symbolic.UnassignablePropsMixin
	_ int
}

func (r *HttpServer) Test(v symbolic.Value, state symbolic.RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	_, ok := v.(*HttpServer)
	return ok
}

func (serv *HttpServer) GetGoMethod(name string) (*symbolic.GoFunction, bool) {
	switch name {
	case "wait_closed":
		return symbolic.WrapGoMethod(serv.wait_closed), true
	case "close":
		return symbolic.WrapGoMethod(serv.close), true
	}
	return nil, false
}

func (s *HttpServer) Prop(name string) symbolic.Value {
	return symbolic.GetGoMethodOrPanic(name, s)
}

func (*HttpServer) PropertyNames() []string {
	return HTTP_SERVER_PROPNAMES
}

func (serv *HttpServer) wait_closed(ctx *symbolic.Context) {
}

func (serv *HttpServer) close(ctx *symbolic.Context) {
}

func (r *HttpServer) PrettyPrint(w prettyprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("http-server")
}

func (r *HttpServer) WidestOfType() symbolic.Value {
	return &HttpServer{}
}
