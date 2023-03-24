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

func (r *ValueHistory) Test(v SymbolicValue) bool {
	switch v.(type) {
	case *ValueHistory:
		return true
	default:
		return false
	}
}

func (r *ValueHistory) WidestOfType() SymbolicValue {
	return &ValueHistory{}
}

func (r *ValueHistory) GetGoMethod(name string) (*GoFunction, bool) {
	switch name {
	case "value_at":
		return WrapGoMethod(r.ValueAt), true
	case "forget_last":
		return WrapGoMethod(r.ForgetLast), true
	}
	return nil, false
}

func (r *ValueHistory) Prop(name string) SymbolicValue {
	switch name {
	case "last_value":
		return ANY
	}
	method, ok := r.GetGoMethod(name)
	if !ok {
		panic(FormatErrPropertyDoesNotExist(name, r))
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

func (ValueHistory *ValueHistory) ValueAt(ctx *Context, d *Date) SymbolicValue {
	return ANY
}

func (ValueHistory *ValueHistory) ForgetLast(ctx *Context) {

}
