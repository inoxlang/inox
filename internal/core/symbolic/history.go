package symbolic

import (
	"errors"

	"github.com/inoxlang/inox/internal/commonfmt"
	pprint "github.com/inoxlang/inox/internal/pretty_print"
)

var (
	VALUE_HISTORY_PROPNAMES = []string{"value_at", "forget_last", "last-value", "selected-datetime", "value-at-selection"}

	ANY_VALUE_HISTORY = &ValueHistory{}
)

// A ValueHistory represents a symbolic ValueHistory.
type ValueHistory struct {
	UnassignablePropsMixin
	_ int
	//TODO: add symbolic value of watched value
}

func NewValueHistory() *ValueHistory {
	return &ValueHistory{}
}

func (h *ValueHistory) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	switch v.(type) {
	case *ValueHistory:
		return true
	default:
		return false
	}
}

func (h *ValueHistory) WidestOfType() Value {
	return &ValueHistory{}
}

func (h *ValueHistory) GetGoMethod(name string) (*GoFunction, bool) {
	switch name {
	case "value_at":
		return WrapGoMethod(h.ValueAt), true
	case "forget_last":
		return WrapGoMethod(h.ForgetLast), true
	}
	return nil, false
}

func (h *ValueHistory) IsSharable() (bool, string) {
	return true, ""
}

func (h *ValueHistory) Share(originState *State) PotentiallySharable {
	return h
}

func (h *ValueHistory) IsShared() bool {
	return true
}

func (h *ValueHistory) Prop(name string) Value {
	switch name {
	case "last-value", "value-at-selection", "selected-datetime":
		return ANY
	}
	method, ok := h.GetGoMethod(name)
	if !ok {
		panic(FormatErrPropertyDoesNotExist(name, h))
	}
	return method
}

func (h *ValueHistory) SetProp(name string, value Value) (IProps, error) {
	switch name {
	case "selected-datetime":
		_, ok := value.(*DateTime)
		if !ok {
			return nil, commonfmt.FmtFailedToSetPropXAcceptXButZProvided(name, "date", Stringify(value))
		}
		return h, nil
	}
	return nil, errors.New("unassignable properties")
}

func (h *ValueHistory) WithExistingPropReplaced(name string, value Value) (IProps, error) {
	return h.SetProp(name, value)
}

func (*ValueHistory) PropertyNames() []string {
	return VALUE_HISTORY_PROPNAMES
}

func (h *ValueHistory) PrettyPrint(w PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("value-history")
	return
}

func (h *ValueHistory) ValueAt(ctx *Context, d *DateTime) Value {
	return ANY
}

func (h *ValueHistory) ForgetLast(ctx *Context) {

}
