package internal

import symbolic "github.com/inox-project/inox/internal/core/symbolic"

type ServerSentEventSource struct {
	symbolic.UnassignablePropsMixin
	_ int
}

func (r *ServerSentEventSource) Test(v symbolic.SymbolicValue) bool {
	_, ok := v.(*ServerSentEventSource)
	return ok
}

func (r ServerSentEventSource) Clone(clones map[uintptr]symbolic.SymbolicValue) symbolic.SymbolicValue {
	return &ServerSentEventSource{}
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
	return []string{"close"}
}

func (serv *ServerSentEventSource) close(ctx *symbolic.Context) {
}

func (r *ServerSentEventSource) Widen() (symbolic.SymbolicValue, bool) {
	return nil, false
}

func (a *ServerSentEventSource) IsWidenable() bool {
	return false
}

func (r *ServerSentEventSource) String() string {
	return "%event-source"
}

func (r *ServerSentEventSource) WidestOfType() symbolic.SymbolicValue {
	return &ServerSentEventSource{}
}
