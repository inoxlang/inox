package symbolic

import (
	"errors"

	"github.com/inoxlang/inox/internal/ast"
	pprint "github.com/inoxlang/inox/internal/prettyprint"
)

var (
	ANY_MAPPING       = &Mapping{}
	MAPPING_PROPNAMES = []string{"compute"}
)

//TODO: implement PotentiallySharable interface

// A Mapping represents a symbolic Mapping.
type Mapping struct {
	shared bool
	SerializableMixin
}

func NewMapping() *Mapping {
	return &Mapping{}
}

func (m *Mapping) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	_, ok := v.(*Mapping)

	return ok
}

func (m *Mapping) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("mapping")
	return
}

func (m *Mapping) WidestOfType() Value {
	return &Mapping{}
}

func (m *Mapping) IteratorElementKey() Value {
	return ANY
}

func (m *Mapping) IteratorElementValue() Value {
	return ANY
}

func (m *Mapping) IsSharable() (bool, string) {
	//TODO: reconcilate with concrete version
	return true, ""
}

func (m *Mapping) Share(originState *State) PotentiallySharable {
	if m.shared {
		return m
	}
	return &Mapping{
		shared: true,
	}
}

func (m *Mapping) IsShared() bool {
	return m.shared
}

func (m *Mapping) GetGoMethod(name string) (*GoFunction, bool) {
	switch name {
	case "compute":
		return WrapGoMethod(m.Compute), true
	}
	return nil, false
}

func (m *Mapping) Prop(name string) Value {
	return GetGoMethodOrPanic(name, m)
}

func (m *Mapping) SetProp(state *State, node ast.Node, name string, value Value) (IProps, error) {
	return nil, errors.New(FmtCannotAssignPropertyOf(m))
}

func (m *Mapping) WithExistingPropReplaced(state *State, name string, value Value) (IProps, error) {
	return nil, errors.New(FmtCannotAssignPropertyOf(m))
}

func (*Mapping) PropertyNames() []string {
	return MAPPING_PROPNAMES
}

func (m *Mapping) Compute(ctx *Context, key Value) Value {
	return ANY
}
