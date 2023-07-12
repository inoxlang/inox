package symbolic

import (
	"bufio"
	"errors"

	pprint "github.com/inoxlang/inox/internal/pretty_print"
	"github.com/inoxlang/inox/internal/utils"
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

func (m *Mapping) Test(v SymbolicValue) bool {
	_, ok := v.(*Mapping)

	return ok
}

func (m *Mapping) Widen() (SymbolicValue, bool) {
	return nil, false
}

func (m *Mapping) IsWidenable() bool {
	return false
}

func (m *Mapping) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%mapping")))
	return
}

func (m *Mapping) WidestOfType() SymbolicValue {
	return &Mapping{}
}

func (m *Mapping) IteratorElementKey() SymbolicValue {
	return ANY
}

func (m *Mapping) IteratorElementValue() SymbolicValue {
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
		return &GoFunction{fn: m.Compute}, true
	}
	return nil, false
}

func (m *Mapping) Prop(name string) SymbolicValue {
	return GetGoMethodOrPanic(name, m)
}

func (m *Mapping) SetProp(name string, value SymbolicValue) (IProps, error) {
	return nil, errors.New(FmtCannotAssignPropertyOf(m))
}

func (m *Mapping) WithExistingPropReplaced(name string, value SymbolicValue) (IProps, error) {
	return nil, errors.New(FmtCannotAssignPropertyOf(m))
}

func (*Mapping) PropertyNames() []string {
	return []string{"compute"}
}

func (m *Mapping) Compute(ctx *Context, key SymbolicValue) SymbolicValue {
	return ANY
}
