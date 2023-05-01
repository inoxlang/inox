package internal

import (
	"bufio"
	"errors"

	parse "github.com/inoxlang/inox/internal/parse"
	pprint "github.com/inoxlang/inox/internal/pretty_print"
	"github.com/inoxlang/inox/internal/utils"
)

var (
	SYMBOLIC_DATA_PROP_NAMES = []string{"errors"}
)

// SymbolicData represents the data produced by the symbolic execution of an AST.
type SymbolicData struct {
	primaryNodeValues        map[parse.Node]SymbolicValue
	secondaryNodeValues      map[parse.Node]SymbolicValue
	localScopeData           map[parse.Node]LocalScopeData
	runtimeTypeCheckPatterns map[parse.Node]any //concrete Pattern or nil (nil means the check is disabled)
	errors                   []SymbolicEvaluationError
}

func NewSymbolicData() *SymbolicData {
	return &SymbolicData{
		primaryNodeValues:        make(map[parse.Node]SymbolicValue, 0),
		secondaryNodeValues:      make(map[parse.Node]SymbolicValue, 0),
		localScopeData:           make(map[parse.Node]LocalScopeData),
		runtimeTypeCheckPatterns: make(map[parse.Node]any, 0),
	}
}

func (data *SymbolicData) IsEmpty() bool {
	return len(data.primaryNodeValues) == 0 && len(data.errors) == 0
}

func (data *SymbolicData) SetNodeValue(node parse.Node, v SymbolicValue) {
	if data == nil {
		return
	}

	_, ok := data.primaryNodeValues[node]
	if ok {
		//TODO:
		//panic(errors.New("node value already set"))
		return
	}

	data.primaryNodeValues[node] = v
}

func (data *SymbolicData) GetNodeValue(node parse.Node) (SymbolicValue, bool) {
	v, ok := data.primaryNodeValues[node]
	return v, ok
}

func (data *SymbolicData) SetSecondaryNodeValue(node parse.Node, v SymbolicValue) {
	if data == nil {
		return
	}

	_, ok := data.secondaryNodeValues[node]
	if ok {
		//TODO:
		//panic(errors.New("secondary node value already set"))
		return
	}

	data.secondaryNodeValues[node] = v
}

func (data *SymbolicData) GetSecondaryNodeValue(node parse.Node) (SymbolicValue, bool) {
	v, ok := data.secondaryNodeValues[node]
	return v, ok
}

func (data *SymbolicData) PushNodeValue(node parse.Node, v SymbolicValue) {
	if data == nil {
		return
	}

	prev, ok := data.primaryNodeValues[node]
	if ok {
		data.primaryNodeValues[node] = v
		data.SetSecondaryNodeValue(node, prev)
		return
	}

	data.primaryNodeValues[node] = v
}

func (data *SymbolicData) SetRuntimeTypecheckPattern(node parse.Node, pattern any) {
	if data == nil {
		return
	}

	_, ok := data.runtimeTypeCheckPatterns[node]
	if ok {
		panic(errors.New("pattern already set"))
	}

	data.runtimeTypeCheckPatterns[node] = pattern
}

func (data *SymbolicData) GetRuntimeTypecheckPattern(node parse.Node) (any, bool) {
	v, ok := data.runtimeTypeCheckPatterns[node]
	return v, ok
}

func (data *SymbolicData) Errors() []SymbolicEvaluationError {
	return data.errors
}

func (data *SymbolicData) AddData(newData *SymbolicData) {
	for k, v := range newData.primaryNodeValues {
		data.SetNodeValue(k, v)
	}

	for k, v := range newData.secondaryNodeValues {
		data.SetSecondaryNodeValue(k, v)
	}

	for k, v := range newData.localScopeData {
		data.SetLocalScopeData(k, v)
	}

	for k, v := range newData.runtimeTypeCheckPatterns {
		data.SetRuntimeTypecheckPattern(k, v)
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

func (d *SymbolicData) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%symbolic-data")))
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

func (d *SymbolicData) GetLocalScopeData(n parse.Node, ancestorChain []parse.Node) (LocalScopeData, bool) {
	if d == nil {
		return LocalScopeData{}, false
	}
	var newAncestorChain []parse.Node

	for {
		scopeData, ok := d.localScopeData[n]
		if ok {
			return scopeData, true
		} else {
			n, newAncestorChain, ok = parse.FindPreviousStatementAndChain(n, ancestorChain)
			if !ok {
				closestBlock, index, ok := parse.FindClosest(ancestorChain, (*parse.Block)(nil))
				if ok && index > 0 && parse.NodeIs(ancestorChain[index-1], (*parse.FunctionExpression)(nil)) {
					return d.GetLocalScopeData(closestBlock, ancestorChain[:index])
				}

				return LocalScopeData{}, false
			}
			ancestorChain = newAncestorChain
		}
	}
}

func (d *SymbolicData) SetLocalScopeData(n parse.Node, scopeData LocalScopeData) {
	if d == nil {
		return
	}

	_, ok := d.localScopeData[n]
	if ok {
		return
	}

	d.localScopeData[n] = scopeData
}

type LocalScopeData struct {
	Variables []LocalVarData
}

type LocalVarData struct {
	Name  string
	Value SymbolicValue
}
