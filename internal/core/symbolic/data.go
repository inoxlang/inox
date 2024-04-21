package symbolic

import (
	"errors"
	"sort"

	"github.com/inoxlang/inox/internal/parse"
	pprint "github.com/inoxlang/inox/internal/prettyprint"
	"github.com/inoxlang/inox/internal/utils"
)

var (
	SYMBOLIC_DATA_PROP_NAMES = []string{"errors"}

	ErrComptimeTypeAlreadyDefined = errors.New("comptile-time type is already defined")
)

// SetLocalScopeData calls s.symbolicData.SetLocalScopeData if we are currently not evaluating an Inox call.
func (s *State) SetLocalScopeData(n parse.Node, scopeData ScopeData) {
	if s.inNonInitialInoxCall() {
		return
	}
	s.symbolicData.SetLocalScopeData(n, scopeData)
}

// SetGlobalScopeData calls s.symbolicData.SetGlobalScopeData if we are currently not evaluating an Inox call.
func (s *State) SetGlobalScopeData(n parse.Node, scopeData ScopeData) {
	if s.inNonInitialInoxCall() {
		return
	}
	s.symbolicData.SetGlobalScopeData(n, scopeData)
}

// SetMostSpecificNodeValue calls s.symbolicData.SetMostSpecificNodeValue if we are currently not evaluating an Inox call.
func (s *State) SetMostSpecificNodeValue(node parse.Node, v Value) {
	if s.inNonInitialInoxCall() {
		return
	}
	s.symbolicData.SetMostSpecificNodeValue(node, v)
}

// Data represents the data produced by the symbolic execution of an AST.
type Data struct {
	mostSpecificNodeValues      map[parse.Node]Value
	lessSpecificNodeValues      map[parse.Node]Value
	expectedNodeValueInfo       map[parse.Node]ExceptedValueInfo
	localScopeData              map[parse.Node]ScopeData
	globalScopeData             map[parse.Node]ScopeData
	contextData                 map[parse.Node]ContextData
	allowedNonPresentProperties map[parse.Node][]string
	allowedNonPresentKeys       map[parse.Node][]string
	runtimeTypeCheckPatterns    map[parse.Node]any //concrete Pattern or nil (nil means the check is disabled)
	usedTypeExtensions          map[*parse.DoubleColonExpression]*TypeExtension
	availableTypeExtensions     map[*parse.DoubleColonExpression][]*TypeExtension
	urlReferencedEntities       map[*parse.DoubleColonExpression]Value
	moduleResults               map[ /* *Chunk or *EmbeddModule */ parse.Node]Value
	comptimeTypes               map[ /* *Chunk or *EmbeddModule */ parse.Node]*ModuleCompileTimeTypes

	errorMessageSet map[string]bool
	errors          []SymbolicEvaluationError

	warningMessageSet map[string]bool
	warnings          []SymbolicEvaluationWarning
}

func NewSymbolicData() *Data {
	return &Data{
		mostSpecificNodeValues:      make(map[parse.Node]Value, 0),
		lessSpecificNodeValues:      make(map[parse.Node]Value, 0),
		localScopeData:              make(map[parse.Node]ScopeData),
		globalScopeData:             make(map[parse.Node]ScopeData),
		allowedNonPresentProperties: make(map[parse.Node][]string, 0),
		allowedNonPresentKeys:       make(map[parse.Node][]string, 0),
		expectedNodeValueInfo:       make(map[parse.Node]ExceptedValueInfo, 0),
		contextData:                 make(map[parse.Node]ContextData),
		runtimeTypeCheckPatterns:    make(map[parse.Node]any, 0),
		usedTypeExtensions:          make(map[*parse.DoubleColonExpression]*TypeExtension, 0),
		availableTypeExtensions:     make(map[*parse.DoubleColonExpression][]*TypeExtension, 0),
		urlReferencedEntities:       make(map[*parse.DoubleColonExpression]Value, 0),
		moduleResults:               make(map[parse.Node]Value, 0),

		comptimeTypes: make(map[parse.Node]*ModuleCompileTimeTypes, 0),

		errorMessageSet:   make(map[string]bool, 0),
		warningMessageSet: make(map[string]bool, 0),
	}
}

func (data *Data) IsEmpty() bool {
	return len(data.mostSpecificNodeValues) == 0 && len(data.errors) == 0
}

func (data *Data) AddError(err SymbolicEvaluationError) {
	if data.errorMessageSet[err.Error()] {
		return
	}
	data.errorMessageSet[err.Error()] = true

	data.errors = append(data.errors, err)
}

func (data *Data) AddWarning(warning SymbolicEvaluationWarning) {
	if warning.LocatedMessage != "" {
		if data.warningMessageSet[warning.LocatedMessage] {
			return
		}
		data.warningMessageSet[warning.LocatedMessage] = true
	}
	data.warnings = append(data.warnings, warning)
}

func (data *Data) SetMostSpecificNodeValue(node parse.Node, v Value) {
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

func (data *Data) GetMostSpecificNodeValue(node parse.Node) (Value, bool) {
	v, ok := data.mostSpecificNodeValues[node]
	return v, ok
}

func (data *Data) SetLessSpecificNodeValue(node parse.Node, v Value) {
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

func (data *Data) GetLessSpecificNodeValue(node parse.Node) (Value, bool) {
	v, ok := data.lessSpecificNodeValues[node]
	return v, ok
}

func (data *Data) PushNodeValue(node parse.Node, v Value) {
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

func (data *Data) SetExpectedNodeValueInfo(node parse.Node, info ExceptedValueInfo) {
	if data == nil {
		return
	}

	_, ok := data.expectedNodeValueInfo[node]
	if ok {
		//TODO:
		//panic(errors.New("node value already set"))
		return
	}

	data.expectedNodeValueInfo[node] = info
}

func (data *Data) GetExpectedNodeValueInfo(node parse.Node) (ExceptedValueInfo, bool) {
	v, ok := data.expectedNodeValueInfo[node]
	return v, ok
}

func (data *Data) SetRuntimeTypecheckPattern(node parse.Node, pattern any) {
	if data == nil {
		return
	}

	_, ok := data.runtimeTypeCheckPatterns[node]
	if ok {
		panic(errors.New("pattern already set"))
	}

	data.runtimeTypeCheckPatterns[node] = pattern
}

func (data *Data) GetRuntimeTypecheckPattern(node parse.Node) (any, bool) {
	v, ok := data.runtimeTypeCheckPatterns[node]
	return v, ok
}

func (data *Data) SetAllowedNonPresentProperties(node parse.Node, properties []string) {
	if data == nil {
		return
	}
	sort.Strings(properties)
	data.allowedNonPresentProperties[node] = properties
}

func (data *Data) GetAllowedNonPresentProperties(node parse.Node) ([]string, bool) {
	v, ok := data.allowedNonPresentProperties[node]
	return v, ok
}

func (data *Data) SetAllowedNonPresentKeys(node parse.Node, keys []string) {
	if data == nil {
		return
	}
	sort.Strings(keys)
	data.allowedNonPresentKeys[node] = keys
}

func (data *Data) GetAllowedNonPresentKeys(node parse.Node) ([]string, bool) {
	v, ok := data.allowedNonPresentKeys[node]
	return v, ok
}

func (data *Data) Errors() []SymbolicEvaluationError {
	return data.errors
}

func (data *Data) Warnings() []SymbolicEvaluationWarning {
	return data.warnings
}

func (data *Data) AddData(newData *Data) {
	for k, v := range newData.mostSpecificNodeValues {
		data.SetMostSpecificNodeValue(k, v)
	}

	for k, v := range newData.lessSpecificNodeValues {
		data.SetLessSpecificNodeValue(k, v)
	}

	for k, v := range newData.expectedNodeValueInfo {
		data.SetExpectedNodeValueInfo(k, v)
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

	for k, v := range newData.usedTypeExtensions {
		data.SetUsedTypeExtension(k, v)
	}

	for k, v := range newData.availableTypeExtensions {
		data.SetAvailableTypeExtensions(k, v)
	}

	for k, v := range newData.urlReferencedEntities {
		data.SetURLReferencedEntity(k, v)
	}

	for k, v := range newData.moduleResults {
		data.SetModuleResult(k, v)
	}

	for k, v := range newData.comptimeTypes {
		data.comptimeTypes[k] = v
	}

	data.errors = append(data.errors, newData.errors...)
	data.warnings = append(data.warnings, newData.warnings...)
}

func (d *Data) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	_, ok := v.(*Data)

	return ok
}

func (d *Data) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("symbolic-data")
}

func (m *Data) WidestOfType() Value {
	return &Data{}
}

func (d *Data) GetGoMethod(name string) (*GoFunction, bool) {
	return nil, false
}

func (d *Data) Prop(name string) Value {
	switch name {
	case "errors":
		return NewTupleOf(NewError(SOURCE_POSITION_RECORD))
	}
	return GetGoMethodOrPanic(name, d)
}

func (d *Data) SetProp(state *State, node parse.Node, name string, value Value) (IProps, error) {
	return nil, errors.New(FmtCannotAssignPropertyOf(d))
}

func (d *Data) WithExistingPropReplaced(state *State, name string, value Value) (IProps, error) {
	return nil, errors.New(FmtCannotAssignPropertyOf(d))
}

func (*Data) PropertyNames() []string {
	return STATIC_CHECK_DATA_PROP_NAMES
}

func (d *Data) Compute(ctx *Context, key Value) Value {
	return ANY
}

func (d *Data) GetLocalScopeData(n parse.Node, ancestorChain []parse.Node) (ScopeData, bool) {
	return d.getScopeData(n, ancestorChain, false)
}

func (d *Data) GetGlobalScopeData(n parse.Node, ancestorChain []parse.Node) (ScopeData, bool) {
	return d.getScopeData(n, ancestorChain, true)
}

func (d *Data) getScopeData(n parse.Node, ancestorChain []parse.Node, global bool) (ScopeData, bool) {
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
				if utils.Implements[*parse.EmbeddedModule](n) {
					return ScopeData{}, false
				}
				lastIndex := len(ancestorChain) - 1
				return d.getScopeData(ancestorChain[lastIndex], ancestorChain[:lastIndex], global)
			} else {
				closestBlock, index, ok := parse.FindClosest(ancestorChain, (*parse.Block)(nil))

				if ok && index > 0 {
					switch ancestor := ancestorChain[index-1].(type) {
					case *parse.FunctionExpression, *parse.ForStatement, *parse.WalkStatement,
						*parse.IfStatement,
						*parse.SwitchStatementCase, *parse.MatchStatementCase, *parse.DefaultCaseWithBlock:
						return d.getScopeData(closestBlock, ancestorChain[:index], global)
					case *parse.ForExpression:
						if _, ok := ancestor.Body.(*parse.Block); ok {
							return d.getScopeData(closestBlock, ancestorChain[:index], global)
						}
					}
				}

				return ScopeData{}, false
			}
		}
	}
}

func (d *Data) SetLocalScopeData(n parse.Node, scopeData ScopeData) {
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
func (d *Data) SetGlobalScopeData(n parse.Node, scopeData ScopeData) {
	if d == nil {
		return
	}

	_, ok := d.globalScopeData[n]
	if ok {
		return
	}

	d.globalScopeData[n] = scopeData
}

func (d *Data) UpdateAllPreviousGlobalScopeDataWithInoxFunction(chunk *parse.Chunk, name string, value *InoxFunction) {
	if d == nil {
		return
	}

	for node, data := range d.globalScopeData {
		for index, varInfo := range data.Variables {
			if _, ok := varInfo.Value.(*inoxFunctionToBeDeclared); ok {
				varInfo.Value = value
				data.Variables[index] = varInfo
				break
			}
		}
		d.globalScopeData[node] = data
	}

}

func (d *Data) SetContextData(n parse.Node, contextData ContextData) {
	if d == nil {
		return
	}

	_, ok := d.contextData[n]
	if ok {
		return
	}

	d.contextData[n] = contextData
}

func (d *Data) GetUsedTypeExtension(n *parse.DoubleColonExpression) (*TypeExtension, bool) {
	e, ok := d.usedTypeExtensions[n]
	return e, ok
}

func (d *Data) SetUsedTypeExtension(n *parse.DoubleColonExpression, ext *TypeExtension) {
	if d == nil {
		return
	}

	_, ok := d.usedTypeExtensions[n]
	if ok {
		panic(errors.New("type extension is already set for this node"))
	}

	d.usedTypeExtensions[n] = ext
}

func (d *Data) GetAvailableTypeExtensions(n *parse.DoubleColonExpression) ([]*TypeExtension, bool) {
	extensions, ok := d.availableTypeExtensions[n]
	return extensions, ok
}

func (d *Data) SetAvailableTypeExtensions(n *parse.DoubleColonExpression, extensions []*TypeExtension) {
	if d == nil {
		return
	}

	_, ok := d.availableTypeExtensions[n]
	if ok {
		panic(errors.New("type extensions are already set for this node"))
	}

	d.availableTypeExtensions[n] = extensions
}

func (d *Data) GetURLReferencedEntity(n *parse.DoubleColonExpression) (Value, bool) {
	value, ok := d.urlReferencedEntities[n]
	return value, ok
}

func (d *Data) SetURLReferencedEntity(n *parse.DoubleColonExpression, value Value) {
	if d == nil {
		return
	}

	_, ok := d.urlReferencedEntities[n]
	if ok {
		panic(errors.New("reference entity is already set for this node"))
	}

	d.urlReferencedEntities[n] = value
}

func (d *Data) SetModuleResult(module parse.Node, value Value) {
	if d == nil {
		return
	}

	switch module.(type) {
	case *parse.Chunk, *parse.EmbeddedModule:
	default:
		panic(errors.New("invalid node"))
	}

	_, ok := d.moduleResults[module]
	if ok {
		panic(errors.New("result is already set for this module"))
	}

	d.moduleResults[module] = value
}

func (d *Data) GetModuleResult(module parse.Node) (Value, bool) {
	if d == nil {
		return nil, false
	}

	switch module.(type) {
	case *parse.Chunk, *parse.EmbeddedModule:
	default:
		panic(errors.New("invalid node"))
	}

	result, ok := d.moduleResults[module]
	return result, ok
}

func (d *Data) GetVariableDefinitionPosition(node parse.Node, ancestors []parse.Node) (pos parse.SourcePositionRange, found bool) {

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

func (d *Data) GetNamedPatternOrPatternNamespacePositionDefinition(node parse.Node, ancestors []parse.Node) (pos parse.SourcePositionRange, found bool) {

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

func (d *Data) GetContextData(n parse.Node, ancestorChain []parse.Node) (ContextData, bool) {
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

			if utils.Implements[*parse.EmbeddedModule](n) {
				return ContextData{}, false
			}
			lastIndex := len(ancestorChain) - 1
			return d.GetContextData(ancestorChain[lastIndex], ancestorChain[:lastIndex])
		}
	}
}

func (d *Data) getModuleComptimeTypes(module parse.Node, create bool) *ModuleCompileTimeTypes {
	switch module.(type) {
	case *parse.Chunk, *parse.EmbeddedModule:
	default:
		panic(errors.New("invalid node"))
	}

	types, ok := d.comptimeTypes[module]
	if create && !ok {
		types = NewModuleCompileTimeTypes()

		d.comptimeTypes[module] = types
	}

	return types

}

func (d *Data) GetCreateComptimeTypes(module parse.Node) *ModuleCompileTimeTypes {
	return d.getModuleComptimeTypes(module, true)
}

func (d *Data) GetComptimeTypes(module parse.Node) (*ModuleCompileTimeTypes, bool) {
	types := d.getModuleComptimeTypes(module, false)
	return types, types != nil
}

type ScopeData struct {
	Variables []VarData
	Chunk     *parse.Chunk
}

type VarData struct {
	Name               string
	Value              Value
	DefinitionPosition parse.SourcePositionRange
}

type ContextData struct {
	Patterns          []NamedPatternData     //the slice is potentially shared between several ContextData
	PatternNamespaces []PatternNamespaceData //the slice is potentially shared between several ContextData
	Extensions        []*TypeExtension
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
