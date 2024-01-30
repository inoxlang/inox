package http_ns

import (
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/prettyprint"
	pprint "github.com/inoxlang/inox/internal/prettyprint"
)

var (
	HTTP_SERVER_PROPNAMES = []string{"wait_closed", "close"}
)

type HttpsServer struct {
	symbolic.UnassignablePropsMixin
	_ int
}

func (r *HttpsServer) Test(v symbolic.Value, state symbolic.RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	_, ok := v.(*HttpsServer)
	return ok
}

func (serv *HttpsServer) GetGoMethod(name string) (*symbolic.GoFunction, bool) {
	switch name {
	case "wait_closed":
		return symbolic.WrapGoMethod(serv.wait_closed), true
	case "close":
		return symbolic.WrapGoMethod(serv.close), true
	}
	return nil, false
}

func (s *HttpsServer) Prop(name string) symbolic.Value {
	return symbolic.GetGoMethodOrPanic(name, s)
}

func (*HttpsServer) PropertyNames() []string {
	return HTTP_SERVER_PROPNAMES
}

func (serv *HttpsServer) wait_closed(ctx *symbolic.Context) {
}

func (serv *HttpsServer) close(ctx *symbolic.Context) {
}

func (r *HttpsServer) PrettyPrint(w prettyprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("http-server")
}

func (r *HttpsServer) WidestOfType() symbolic.Value {
	return &HttpsServer{}
}
