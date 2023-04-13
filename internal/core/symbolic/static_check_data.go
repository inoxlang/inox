package internal

import "errors"

var (
	STATIC_CHECK_DATA_PROP_NAMES = []string{"errors"}
)

// A StaticCheckData represents a symbolic StaticCheckData.
type StaticCheckData struct {
	_ int
}

func (d *StaticCheckData) Test(v SymbolicValue) bool {
	_, ok := v.(*StaticCheckData)

	return ok
}

func (d *StaticCheckData) Widen() (SymbolicValue, bool) {
	return nil, false
}

func (d *StaticCheckData) IsWidenable() bool {
	return false
}

func (d *StaticCheckData) String() string {
	return "%static-check-data"
}

func (m *StaticCheckData) WidestOfType() SymbolicValue {
	return &StaticCheckData{}
}

func (d *StaticCheckData) GetGoMethod(name string) (*GoFunction, bool) {
	return nil, false
}

func (d *StaticCheckData) Prop(name string) SymbolicValue {
	switch name {
	case "errors":
		return NewTupleOf(NewError(SOURCE_POSITION_RECORD))
	}
	return GetGoMethodOrPanic(name, d)
}

func (d *StaticCheckData) SetProp(name string, value SymbolicValue) (IProps, error) {
	return nil, errors.New(FmtCannotAssignPropertyOf(d))
}

func (d *StaticCheckData) WithExistingPropReplaced(name string, value SymbolicValue) (IProps, error) {
	return nil, errors.New(FmtCannotAssignPropertyOf(d))
}

func (*StaticCheckData) PropertyNames() []string {
	return STATIC_CHECK_DATA_PROP_NAMES
}

func (d *StaticCheckData) Compute(ctx *Context, key SymbolicValue) SymbolicValue {
	return ANY
}
