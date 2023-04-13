package internal

import (
	"errors"

	parse "github.com/inox-project/inox/internal/parse"
)

var (
	SYMBOLIC_DATA_PROP_NAMES = []string{"errors"}
)

// SymbolicData represents the data produced by the symbolic execution of an AST.
type SymbolicData struct {
	nodeMap map[parse.Node]SymbolicValue
	errors  []SymbolicEvaluationError
}

func NewSymbolicData() *SymbolicData {
	return &SymbolicData{
		nodeMap: make(map[parse.Node]SymbolicValue, 0),
	}
}

func (data *SymbolicData) SetNodeValue(node parse.Node, v SymbolicValue) {
	_, ok := data.nodeMap[node]
	if ok {
		//data.nodeMap[node] = ANY
		return
	}

	data.nodeMap[node] = v
}

func (data *SymbolicData) GetNodeValue(node parse.Node) (SymbolicValue, bool) {
	v, ok := data.nodeMap[node]
	return v, ok
}

func (data *SymbolicData) Errors() []SymbolicEvaluationError {
	return data.errors
}

func (data *SymbolicData) AddData(newData *SymbolicData) {
	for k, v := range newData.nodeMap {
		data.SetNodeValue(k, v)
	}

	data.errors = append(data.errors, newData.errors...)
}

func (d *SymbolicData) Test(v SymbolicValue) bool {
	_, ok := v.(*SymbolicData)

	return ok
}

func (d *SymbolicData) Widen() (SymbolicValue, bool) {
	return nil, false
}

func (d *SymbolicData) IsWidenable() bool {
	return false
}

func (d *SymbolicData) String() string {
	return "%symbolic-data"
}

func (m *SymbolicData) WidestOfType() SymbolicValue {
	return &SymbolicData{}
}

func (d *SymbolicData) GetGoMethod(name string) (*GoFunction, bool) {
	return nil, false
}

func (d *SymbolicData) Prop(name string) SymbolicValue {
	switch name {
	case "errors":
		return NewTupleOf(NewError(SOURCE_POSITION_RECORD))
	}
	return GetGoMethodOrPanic(name, d)
}

func (d *SymbolicData) SetProp(name string, value SymbolicValue) (IProps, error) {
	return nil, errors.New(FmtCannotAssignPropertyOf(d))
}

func (d *SymbolicData) WithExistingPropReplaced(name string, value SymbolicValue) (IProps, error) {
	return nil, errors.New(FmtCannotAssignPropertyOf(d))
}

func (*SymbolicData) PropertyNames() []string {
	return STATIC_CHECK_DATA_PROP_NAMES
}

func (d *SymbolicData) Compute(ctx *Context, key SymbolicValue) SymbolicValue {
	return ANY
}
