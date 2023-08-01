package symbolic

import (
	"bufio"
	"errors"

	"github.com/inoxlang/inox/internal/commonfmt"
	pprint "github.com/inoxlang/inox/internal/pretty_print"
	"github.com/inoxlang/inox/internal/utils"
)

var (
	VALUE_HISTORY_PROPNAMES = []string{"value_at", "forget_last", "last-value", "selected-date", "value-at-selection"}

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

func (h *ValueHistory) Test(v SymbolicValue) bool {
	switch v.(type) {
	case *ValueHistory:
		return true
	default:
		return false
	}
}

func (h *ValueHistory) WidestOfType() SymbolicValue {
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

func (h *ValueHistory) Prop(name string) SymbolicValue {
	switch name {
	case "last-value", "value-at-selection", "selected-date":
		return ANY
	}
	method, ok := h.GetGoMethod(name)
	if !ok {
		panic(FormatErrPropertyDoesNotExist(name, h))
	}
	return method
}

func (h *ValueHistory) SetProp(name string, value SymbolicValue) (IProps, error) {
	switch name {
	case "selected-date":
		_, ok := value.(*Date)
		if !ok {
			return nil, commonfmt.FmtFailedToSetPropXAcceptXButZProvided(name, "date", Stringify(value))
		}
		return h, nil
	}
	return nil, errors.New("unassignable properties")
}

func (h *ValueHistory) WithExistingPropReplaced(name string, value SymbolicValue) (IProps, error) {
	return h.SetProp(name, value)
}

func (*ValueHistory) PropertyNames() []string {
	return VALUE_HISTORY_PROPNAMES
}

func (h *ValueHistory) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%value-history")))
	return
}

func (h *ValueHistory) ValueAt(ctx *Context, d *Date) SymbolicValue {
	return ANY
}

func (h *ValueHistory) ForgetLast(ctx *Context) {

}
