package http_ns

import (
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/prettyprint"
	pprint "github.com/inoxlang/inox/internal/prettyprint"
)

var (
	SSE_SOURCE_PROPNAMES = []string{"close"}
)

type ServerSentEventSource struct {
	symbolic.UnassignablePropsMixin
	_ int
}

func (r *ServerSentEventSource) Test(v symbolic.Value, state symbolic.RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	_, ok := v.(*ServerSentEventSource)
	return ok
}

func (serv *ServerSentEventSource) GetGoMethod(name string) (*symbolic.GoFunction, bool) {
	switch name {
	case "close":
		return symbolic.WrapGoMethod(serv.close), true
	}
	return nil, false
}

func (s *ServerSentEventSource) Prop(name string) symbolic.Value {
	return symbolic.GetGoMethodOrPanic(name, s)
}

func (*ServerSentEventSource) PropertyNames() []string {
	return SSE_SOURCE_PROPNAMES
}

func (serv *ServerSentEventSource) close(ctx *symbolic.Context) {
}

func (r *ServerSentEventSource) PrettyPrint(w prettyprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("event-source")
}

func (r *ServerSentEventSource) WidestOfType() symbolic.Value {
	return &ServerSentEventSource{}
}
