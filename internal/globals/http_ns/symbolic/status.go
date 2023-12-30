package http_ns

import (
	"github.com/inoxlang/inox/internal/core/symbolic"
	pprint "github.com/inoxlang/inox/internal/prettyprint"
)

var (
	STATUS_PROPNAMES = []string{"code", "full-text"}
	ANY_STATUS       = &Status{}
	ANY_STATUS_CODE  = &StatusCode{}

	STATUS_CODE_INT_RANGE = symbolic.NewIncludedEndIntRange(symbolic.NewInt(100), symbolic.NewInt(599))
)

type Status struct {
	symbolic.UnassignablePropsMixin
	symbolic.SerializableMixin
}

func (s *Status) Test(v symbolic.Value, state symbolic.RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	_, ok := v.(*Status)
	return ok
}

func (*Status) GetGoMethod(name string) (*symbolic.GoFunction, bool) {
	return nil, false
}

func (s *Status) Prop(name string) symbolic.Value {
	switch name {
	case "code":
		return ANY_STATUS_CODE
	case "full-text":
		return symbolic.ANY_STR
	default:
		return symbolic.GetGoMethodOrPanic(name, s)
	}
}

func (Status) PropertyNames() []string {
	return STATUS_PROPNAMES
}

func (r *Status) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("http.status")
}

func (s *Status) WidestOfType() symbolic.Value {
	return ANY_STATUS
}

type StatusCode struct {
	symbolic.SerializableMixin
}

func (s *StatusCode) Test(v symbolic.Value, state symbolic.RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	_, ok := v.(*StatusCode)
	return ok
}

func (c *StatusCode) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("http.status-code")
}

func (c *StatusCode) WidestOfType() symbolic.Value {
	return ANY_STATUS_CODE
}
