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
	nodeMap                  map[parse.Node]SymbolicValue
	localScopeData           map[parse.Node]LocalScopeData
	runtimeTypeCheckPatterns map[parse.Node]any //concrete Pattern
	errors                   []SymbolicEvaluationError
}

func NewSymbolicData() *SymbolicData {
	return &SymbolicData{
		nodeMap:                  make(map[parse.Node]SymbolicValue, 0),
		localScopeData:           make(map[parse.Node]LocalScopeData),
		runtimeTypeCheckPatterns: make(map[parse.Node]any, 0),
	}
}

func (data *SymbolicData) IsEmpty() bool {
	return len(data.nodeMap) == 0 && len(data.errors) == 0
}

func (data *SymbolicData) SetNodeValue(node parse.Node, v SymbolicValue) {
	if data == nil {
		return
	}

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
	for k, v := range newData.nodeMap {
		data.SetNodeValue(k, v)
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

	for {
		scopeData, ok := d.localScopeData[n]
		if ok {
			return scopeData, true
		} else {
			n, ancestorChain, ok = parse.FindPreviousStatementAndChain(n, ancestorChain)
			if !ok {
				return LocalScopeData{}, false
			}
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
