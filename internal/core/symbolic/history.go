package internal

var (
	VALUE_HISTORY_PROPNAMES = []string{"value_at", "forget_last", "last_value"}

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
	case "last_value":
		return ANY
	}
	method, ok := h.GetGoMethod(name)
	if !ok {
		panic(FormatErrPropertyDoesNotExist(name, h))
	}
	return method
}

func (*ValueHistory) PropertyNames() []string {
	return VALUE_HISTORY_PROPNAMES
}

func (h *ValueHistory) Widen() (SymbolicValue, bool) {
	return nil, false
}

func (h *ValueHistory) IsWidenable() bool {
	return false
}

func (h *ValueHistory) String() string {
	return "value-history"
}

func (h *ValueHistory) ValueAt(ctx *Context, d *Date) SymbolicValue {
	return ANY
}

func (h *ValueHistory) ForgetLast(ctx *Context) {

}
