package http_ns

import (
	"bufio"

	"github.com/inoxlang/inox/internal/core/symbolic"
	pprint "github.com/inoxlang/inox/internal/pretty_print"

	"github.com/inoxlang/inox/internal/utils"
)

var (
	SSE_SOURCE_PROPNAMES = []string{"close"}
)

type ServerSentEventSource struct {
	symbolic.UnassignablePropsMixin
	_ int
}

func (r *ServerSentEventSource) Test(v symbolic.SymbolicValue, state symbolic.RecTestCallState) bool {
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

func (s *ServerSentEventSource) Prop(name string) symbolic.SymbolicValue {
	return symbolic.GetGoMethodOrPanic(name, s)
}

func (*ServerSentEventSource) PropertyNames() []string {
	return SSE_SOURCE_PROPNAMES
}

func (serv *ServerSentEventSource) close(ctx *symbolic.Context) {
}

func (r *ServerSentEventSource) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%event-source")))
	return
}

func (r *ServerSentEventSource) WidestOfType() symbolic.SymbolicValue {
	return &ServerSentEventSource{}
}
