package symbolic

import (
	"bufio"
	"errors"
	"sort"

	parse "github.com/inoxlang/inox/internal/parse"
	pprint "github.com/inoxlang/inox/internal/pretty_print"
	"github.com/inoxlang/inox/internal/utils"
)

var (
	SYMBOLIC_DATA_PROP_NAMES = []string{"errors"}
)

// SymbolicData represents the data produced by the symbolic execution of an AST.
type SymbolicData struct {
	mostSpecificNodeValues      map[parse.Node]SymbolicValue
	lessSpecificNodeValues      map[parse.Node]SymbolicValue
	localScopeData              map[parse.Node]ScopeData
	globalScopeData             map[parse.Node]ScopeData
	contextData                 map[parse.Node]ContextData
	allowedNonPresentProperties map[parse.Node][]string
	allowedNonPresentKeys       map[parse.Node][]string
	runtimeTypeCheckPatterns    map[parse.Node]any //concrete Pattern or nil (nil means the check is disabled)

	errorMessageSet map[string]bool
	errors          []SymbolicEvaluationError
	warnings        []SymbolicEvaluationWarning
}

func NewSymbolicData() *SymbolicData {
	return &SymbolicData{
		mostSpecificNodeValues:      make(map[parse.Node]SymbolicValue, 0),
		lessSpecificNodeValues:      make(map[parse.Node]SymbolicValue, 0),
		localScopeData:              make(map[parse.Node]ScopeData),
		globalScopeData:             make(map[parse.Node]ScopeData),
		allowedNonPresentProperties: make(map[parse.Node][]string, 0),
		allowedNonPresentKeys:       make(map[parse.Node][]string),
		contextData:                 make(map[parse.Node]ContextData),
		runtimeTypeCheckPatterns:    make(map[parse.Node]any, 0),

		errorMessageSet: make(map[string]bool, 0),
	}
}

func (data *SymbolicData) IsEmpty() bool {
	return len(data.mostSpecificNodeValues) == 0 && len(data.errors) == 0
}

func (data *SymbolicData) AddError(err SymbolicEvaluationError) {
	if data.errorMessageSet[err.Error()] {
		return
	}
	data.errorMessageSet[err.Error()] = true

	data.errors = append(data.errors, err)
}

func (data *SymbolicData) AddWarning(warning SymbolicEvaluationWarning) {
	data.warnings = append(data.warnings, warning)
}

func (data *SymbolicData) SetMostSpecificNodeValue(node parse.Node, v SymbolicValue) {
	if data == nil {
		return
	}

	_, ok := data.mostSpecificNodeValues[node]
	if ok {
		//TODO:
		//panic(errors.New("node value already set"))
		return
	}

	data.mostSpecificNodeValues[node] = v
}

func (data *SymbolicData) GetMostSpecificNodeValue(node parse.Node) (SymbolicValue, bool) {
	v, ok := data.mostSpecificNodeValues[node]
	return v, ok
}

func (data *SymbolicData) SetLessSpecificNodeValue(node parse.Node, v SymbolicValue) {
	if data == nil {
		return
	}

	_, ok := data.lessSpecificNodeValues[node]
	if ok {
		//TODO:
		//panic(errors.New("secondary node value already set"))
		return
	}

	data.lessSpecificNodeValues[node] = v
}

func (data *SymbolicData) GetLessSpecificNodeValue(node parse.Node) (SymbolicValue, bool) {
	v, ok := data.lessSpecificNodeValues[node]
	return v, ok
}

func (data *SymbolicData) PushNodeValue(node parse.Node, v SymbolicValue) {
	if data == nil {
		return
	}

	prev, ok := data.mostSpecificNodeValues[node]
	if ok {
		data.mostSpecificNodeValues[node] = v
		data.SetLessSpecificNodeValue(node, prev)
		return
	}

	data.mostSpecificNodeValues[node] = v
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

func (data *SymbolicData) SetAllowedNonPresentProperties(node parse.Node, properties []string) {
	if data == nil {
		return
	}
	sort.Strings(properties)
	data.allowedNonPresentProperties[node] = properties
}

func (data *SymbolicData) GetAllowedNonPresentProperties(node parse.Node) ([]string, bool) {
	v, ok := data.allowedNonPresentProperties[node]
	return v, ok
}

func (data *SymbolicData) SetAllowedNonPresentKeys(node parse.Node, keys []string) {
	if data == nil {
		return
	}
	sort.Strings(keys)
	data.allowedNonPresentKeys[node] = keys
}

func (data *SymbolicData) GetAllowedNonPresentKeys(node parse.Node) ([]string, bool) {
	v, ok := data.allowedNonPresentKeys[node]
	return v, ok
}

func (data *SymbolicData) Errors() []SymbolicEvaluationError {
	return data.errors
}

func (data *SymbolicData) Warnings() []SymbolicEvaluationWarning {
	return data.warnings
}

func (data *SymbolicData) AddData(newData *SymbolicData) {
	for k, v := range newData.mostSpecificNodeValues {
		data.SetMostSpecificNodeValue(k, v)
	}

	for k, v := range newData.lessSpecificNodeValues {
		data.SetLessSpecificNodeValue(k, v)
	}

	for k, v := range newData.localScopeData {
		data.SetLocalScopeData(k, v)
	}

	for k, v := range newData.globalScopeData {
		data.SetGlobalScopeData(k, v)
	}

	for k, v := range newData.contextData {
		data.SetContextData(k, v)
	}

	for k, v := range newData.allowedNonPresentProperties {
		data.SetAllowedNonPresentProperties(k, v)
	}

	for k, v := range newData.allowedNonPresentKeys {
		data.SetAllowedNonPresentKeys(k, v)
	}

	for k, v := range newData.runtimeTypeCheckPatterns {
		data.SetRuntimeTypecheckPattern(k, v)
	}

	data.errors = append(data.errors, newData.errors...)
	data.warnings = append(data.warnings, newData.warnings...)
}

func (d *SymbolicData) Test(v SymbolicValue) bool {
	_, ok := v.(*SymbolicData)

	return ok
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

func (d *SymbolicData) GetLocalScopeData(n parse.Node, ancestorChain []parse.Node) (ScopeData, bool) {
	return d.getScopeData(n, ancestorChain, false)
}

func (d *SymbolicData) GetGlobalScopeData(n parse.Node, ancestorChain []parse.Node) (ScopeData, bool) {
	return d.getScopeData(n, ancestorChain, true)
}

func (d *SymbolicData) getScopeData(n parse.Node, ancestorChain []parse.Node, global bool) (ScopeData, bool) {
	if d == nil {
		return ScopeData{}, false
	}
	var newAncestorChain []parse.Node

	for {
		var scopeData ScopeData
		var ok bool
		if global {
			scopeData, ok = d.globalScopeData[n]
		} else {
			scopeData, ok = d.localScopeData[n]
		}

		if ok {
			return scopeData, true
		} else {
			n, newAncestorChain, ok = parse.FindPreviousStatementAndChain(n, ancestorChain, false)
			if ok {
				ancestorChain = newAncestorChain
				continue
			}

			if len(ancestorChain) == 0 {
				return ScopeData{}, false
			}

			if global {
				if parse.NodeIs(n, (*parse.EmbeddedModule)(nil)) {
					return ScopeData{}, false
				}
				lastIndex := len(ancestorChain) - 1
				return d.getScopeData(ancestorChain[lastIndex], ancestorChain[:lastIndex], global)
			} else {
				closestBlock, index, ok := parse.FindClosest(ancestorChain, (*parse.Block)(nil))

				if ok && index > 0 {
					switch ancestorChain[index-1].(type) {
					case *parse.FunctionExpression, *parse.ForStatement, *parse.WalkStatement:
						return d.getScopeData(closestBlock, ancestorChain[:index], global)
					}
				}

				return ScopeData{}, false
			}
		}
	}
}

func (d *SymbolicData) SetLocalScopeData(n parse.Node, scopeData ScopeData) {
	if d == nil {
		return
	}

	_, ok := d.localScopeData[n]
	if ok {
		return
	}

	d.localScopeData[n] = scopeData
}

// TODO: global scope data generally contain a lot of variables, find a way to reduce memory usage.
func (d *SymbolicData) SetGlobalScopeData(n parse.Node, scopeData ScopeData) {
	if d == nil {
		return
	}

	_, ok := d.globalScopeData[n]
	if ok {
		return
	}

	d.globalScopeData[n] = scopeData
}

func (d *SymbolicData) SetContextData(n parse.Node, contextData ContextData) {
	if d == nil {
		return
	}

	_, ok := d.contextData[n]
	if ok {
		return
	}

	d.contextData[n] = contextData
}

func (d *SymbolicData) GetVariableDefinitionPosition(node parse.Node, ancestors []parse.Node) (pos parse.SourcePositionRange, found bool) {

	var data ScopeData
	var ok bool
	var name string

switch_:
	switch node := node.(type) {
	case *parse.IdentifierLiteral:
		name = node.Name
		data, ok = d.GetGlobalScopeData(node, ancestors)

		if ok {
			for _, varInfo := range data.Variables {
				if varInfo.Name == name {
					break switch_
				}
			}
		}

		data, ok = d.GetLocalScopeData(node, ancestors)
		if !ok {
			return
		}
	case *parse.Variable:
		name = node.Name
		data, ok = d.GetLocalScopeData(node, ancestors)
		if !ok {
			return
		}
	case *parse.GlobalVariable:
		name = node.Name
		data, ok = d.GetGlobalScopeData(node, ancestors)
		if !ok {
			return
		}
	default:
		return
	}

	for _, varInfo := range data.Variables {
		if varInfo.Name == name && (varInfo.DefinitionPosition != parse.SourcePositionRange{}) {
			pos = varInfo.DefinitionPosition
			found = true
			return
		}
	}

	found = false
	return
}

func (d *SymbolicData) GetNamedPatternOrPatternNamespacePositionDefinition(node parse.Node, ancestors []parse.Node) (pos parse.SourcePositionRange, found bool) {

	switch node := node.(type) {
	case *parse.PatternIdentifierLiteral:
		data, ok := d.GetContextData(node, ancestors)
		if !ok {
			return
		}
		for _, e := range data.Patterns {
			if e.Name == node.Name && e.DefinitionPosition != (parse.SourcePositionRange{}) {
				return e.DefinitionPosition, true
			}
		}
	case *parse.PatternNamespaceIdentifierLiteral:
		data, ok := d.GetContextData(node, ancestors)
		if !ok {
			return
		}
		for _, e := range data.PatternNamespaces {
			if e.Name == node.Name && e.DefinitionPosition != (parse.SourcePositionRange{}) {
				return e.DefinitionPosition, true
			}
		}
	default:
		return
	}

	found = false
	return
}

func (d *SymbolicData) GetContextData(n parse.Node, ancestorChain []parse.Node) (ContextData, bool) {
	if d == nil {
		return ContextData{}, false
	}
	var newAncestorChain []parse.Node

	for {
		contextData, ok := d.contextData[n]

		if ok {
			return contextData, true
		} else {
			n, newAncestorChain, ok = parse.FindPreviousStatementAndChain(n, ancestorChain, true)
			if ok {
				ancestorChain = newAncestorChain
				continue
			}

			if len(ancestorChain) == 0 {
				return ContextData{}, false
			}

			if parse.NodeIs(n, (*parse.EmbeddedModule)(nil)) {
				return ContextData{}, false
			}
			lastIndex := len(ancestorChain) - 1
			return d.GetContextData(ancestorChain[lastIndex], ancestorChain[:lastIndex])
		}
	}
}

type ScopeData struct {
	Variables []VarData
}

type VarData struct {
	Name               string
	Value              SymbolicValue
	DefinitionPosition parse.SourcePositionRange
}

type ContextData struct {
	Patterns          []NamedPatternData     //the slice is potentially shared between several ContextData
	PatternNamespaces []PatternNamespaceData //the slice is potentially shared between several ContextData
}

type NamedPatternData struct {
	Name               string
	Value              Pattern
	DefinitionPosition parse.SourcePositionRange
}

type PatternNamespaceData struct {
	Name               string
	Value              *PatternNamespace
	DefinitionPosition parse.SourcePositionRange
}
