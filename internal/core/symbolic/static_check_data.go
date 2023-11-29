package symbolic

import (
	"errors"

	pprint "github.com/inoxlang/inox/internal/prettyprint"
)

var (
	STATIC_CHECK_DATA_PROP_NAMES = []string{"errors"}
)

// A StaticCheckData represents a symbolic StaticCheckData.
type StaticCheckData struct {
	_ int
}

func (d *StaticCheckData) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	_, ok := v.(*StaticCheckData)

	return ok
}

func (d *StaticCheckData) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("static-check-data")
	return
}

func (m *StaticCheckData) WidestOfType() Value {
	return &StaticCheckData{}
}

func (d *StaticCheckData) GetGoMethod(name string) (*GoFunction, bool) {
	return nil, false
}

func (d *StaticCheckData) Prop(name string) Value {
	switch name {
	case "errors":
		return NewTupleOf(NewError(SOURCE_POSITION_RECORD))
	}
	return GetGoMethodOrPanic(name, d)
}

func (d *StaticCheckData) SetProp(name string, value Value) (IProps, error) {
	return nil, errors.New(FmtCannotAssignPropertyOf(d))
}

func (d *StaticCheckData) WithExistingPropReplaced(name string, value Value) (IProps, error) {
	return nil, errors.New(FmtCannotAssignPropertyOf(d))
}

func (*StaticCheckData) PropertyNames() []string {
	return STATIC_CHECK_DATA_PROP_NAMES
}

func (d *StaticCheckData) Compute(ctx *Context, key Value) Value {
	return ANY
}
