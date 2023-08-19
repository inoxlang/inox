package symbolic

import (
	"errors"

	parse "github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/utils"
)

// State is the state of a symbolic evaluation.
// TODO: reduce memory usage of scopes
type State struct {
	parent *State //can be nil

	ctx        *Context
	chunkStack []*parse.ParsedChunk
	//positions of module/chunk import statements
	importPositions []parse.SourcePositionRange

	// first scope is the global scope, forks start with a global scope copy & a copy of the deepest local scope
	scopeStack            []*scopeInfo
	inPreinit             bool
	recursiveFunctionName string

	calleeStack       []*parse.FunctionExpression
	topLevelSelf      SymbolicValue // can be nil
	returnType        SymbolicValue
	returnValue       SymbolicValue
	conditionalReturn bool
	iterationChange   IterationChange
	Module            *Module

	baseGlobals           map[string]SymbolicValue
	basePatterns          map[string]Pattern
	basePatternNamespaces map[string]*PatternNamespace

	tempSymbolicGoFunctionErrors         []string
	tempSymbolicGoFunctionParameters     *[]SymbolicValue
	tempSymbolicGoFunctionParameterNames []string
	tempUpdatedSelf                      SymbolicValue

	lastErrorNode parse.Node
	symbolicData  *SymbolicData
}

type scopeInfo struct {
	self      SymbolicValue //can be nil
	nextSelf  SymbolicValue //can be nil
	variables map[string]varSymbolicInfo
}

type tempSymbolicGoFunctionSignature struct {
	params      []SymbolicValue
	returnTypes []SymbolicValue
}

func newSymbolicState(ctx *Context, chunk *parse.ParsedChunk) *State {
	chunkStack := []*parse.ParsedChunk{chunk}

	if ctx.associatedState != nil {
		panic(errors.New("cannot create new state: passed context already has an associated state"))
	}

	state := &State{
		parent:     nil,
		ctx:        ctx,
		chunkStack: chunkStack,
		scopeStack: []*scopeInfo{
			{variables: map[string]varSymbolicInfo{}},
		},
		returnValue:     nil,
		iterationChange: NoIterationChange,
	}
	ctx.associatedState = state

	return state
}

func (state *State) getErrorMesssageLocation(node parse.Node) parse.SourcePositionStack {
	sourcePositionStack := utils.CopySlice(state.importPositions)
	sourcePositionStack = append(sourcePositionStack, state.currentChunk().GetSourcePosition(node.Base().Span))
	return sourcePositionStack
}

func (state *State) topChunk() *parse.ParsedChunk {
	if len(state.chunkStack) == 0 {
		return nil
	}
	return state.chunkStack[0]
}

func (state *State) currentChunk() *parse.ParsedChunk {
	if len(state.chunkStack) == 0 {
		state.chunkStack = append(state.chunkStack, state.Module.mainChunk)
	}
	return state.chunkStack[len(state.chunkStack)-1]
}

func (state *State) pushChunk(chunk *parse.ParsedChunk, stmt *parse.InclusionImportStatement) {
	state.importPositions = append(state.importPositions, state.currentChunk().GetSourcePosition(stmt.Span))
	state.chunkStack = append(state.chunkStack, chunk)
}

func (state *State) popChunk() {
	state.chunkStack = state.chunkStack[:len(state.chunkStack)-1]
	state.importPositions = state.importPositions[:len(state.importPositions)-1]
}

func (state *State) assertHasLocals() {
	if len(state.scopeStack) <= 1 {
		panic("no locals")
	}
}

func (state *State) globalCount() int {
	return len(state.scopeStack[0].variables)
}

func (state *State) localCount() int {
	state.assertHasLocals()
	return len(state.scopeStack[len(state.scopeStack)-1].variables)
}

func (state *State) setGlobal(name string, value SymbolicValue, constness GlobalConstness, optDefinitionNode ...parse.Node) (ok bool) {
	scope := state.scopeStack[0]
	var info varSymbolicInfo

	if info_, alreadyDefined := scope.variables[name]; alreadyDefined {
		info = info_
		info.value = value
	} else {
		var definitionPosition parse.SourcePositionRange
		if len(optDefinitionNode) != 0 {
			definitionPosition = state.getCurrentChunkNodePositionOrZero(optDefinitionNode[0])
		}

		info = varSymbolicInfo{
			isConstant:         constness == GlobalConst,
			static:             getStatic(value),
			value:              value,
			definitionPosition: definitionPosition,
		}
		scope.variables[name] = info
		return true
	}

	if info.isConstant {
		return false
	}

	scope.variables[name] = info
	return true
}

func (state *State) overrideGlobal(name string, value SymbolicValue) (ok bool) {
	scope := state.scopeStack[0]
	info := scope.variables[name]
	info.value = value
	scope.variables[name] = info
	return true
}

func (state *State) setLocal(name string, value SymbolicValue, static Pattern, optDefinitionNode ...parse.Node) {
	state.assertHasLocals()
	scope := state.scopeStack[len(state.scopeStack)-1]

	if static == nil {
		static = getStatic(value)
	}

	var definitionPosition parse.SourcePositionRange
	if len(optDefinitionNode) != 0 {
		definitionPosition = state.getCurrentChunkNodePositionOrZero(optDefinitionNode[0])
	}

	scope.variables[name] = varSymbolicInfo{
		value:              value,
		static:             static,
		definitionPosition: definitionPosition,
	}
}

func (state *State) getCurrentChunkNodePositionOrZero(node parse.Node) parse.SourcePositionRange {
	return state.currentChunk().GetSourcePosition(node.Base().Span)
}

func (state *State) overrideLocal(name string, value SymbolicValue) {
	state.assertHasLocals()
	scope := state.scopeStack[len(state.scopeStack)-1]

	static := &TypePattern{val: value.WidestOfType()}

	scope.variables[name] = varSymbolicInfo{
		value:  value,
		static: static,
	}
}

func (state *State) removeLocal(name string) {
	state.assertHasLocals()
	scope := state.scopeStack[len(state.scopeStack)-1]

	delete(scope.variables, name)
}

func (state *State) setNextSelf(value SymbolicValue) {
	state.assertHasLocals()
	scope := state.scopeStack[len(state.scopeStack)-1]

	if scope.nextSelf != nil {
		panic("next self is already set")
	}

	scope.nextSelf = value
}

func (state *State) unsetNextSelf() {
	state.assertHasLocals()
	scope := state.scopeStack[len(state.scopeStack)-1]

	if scope.nextSelf == nil {
		panic("next self is already unset")
	}

	scope.nextSelf = nil
}

func (state *State) getNextSelf() (SymbolicValue, bool) {
	state.assertHasLocals()
	scope := state.scopeStack[len(state.scopeStack)-1]

	return scope.nextSelf, scope.nextSelf != nil
}

func (state *State) setSelf(value SymbolicValue) {
	state.assertHasLocals()
	scope := state.scopeStack[len(state.scopeStack)-1]

	if scope.self != nil {
		panic("self is already set")
	}

	scope.self = value
}

func (state *State) unsetSelf() {
	state.assertHasLocals()
	scope := state.scopeStack[len(state.scopeStack)-1]

	if scope.self == nil {
		panic("self is already unset")
	}

	scope.self = nil
}

func (state *State) getSelf() (SymbolicValue, bool) {
	state.assertHasLocals()
	scope := state.scopeStack[len(state.scopeStack)-1]
	return scope.self, scope.self != nil
}

func (state *State) hasGlobal(name string) bool {
	_, ok := state.scopeStack[0].variables[name]
	return ok || (state.parent != nil && state.parent.hasGlobal(name))
}

func (state *State) getGlobal(name string) (varSymbolicInfo, bool) {
	scope := state.scopeStack[0]
	if v, ok := scope.variables[name]; ok {
		return v, true
	}
	return varSymbolicInfo{}, false
}

func (state *State) forEachGlobal(fn func(name string, info varSymbolicInfo)) {
	state.assertHasLocals()

	for k, v := range state.scopeStack[0].variables {
		fn(k, v)
	}
}

func (state *State) hasLocal(name string) bool {
	if len(state.scopeStack) <= 1 {
		return false
	}

	scope := state.scopeStack[len(state.scopeStack)-1]
	_, ok := scope.variables[name]
	return ok
}

func (state *State) getLocal(name string) (varSymbolicInfo, bool) {
	state.assertHasLocals()

	scope := state.scopeStack[len(state.scopeStack)-1]
	if v, ok := scope.variables[name]; ok {
		return v, true
	}
	return varSymbolicInfo{}, false
}

func (state *State) get(name string) (varSymbolicInfo, bool) {
	if state.hasLocal(name) {
		return state.getLocal(name)
	}
	return state.getGlobal(name)
}

func (state *State) updateLocal(name string, value SymbolicValue, node parse.Node) bool {
	ok, _ := state.updateLocal2(name, node, func(expected SymbolicValue) (SymbolicValue, bool, error) {
		return value, false, nil
	}, false)
	return ok
}

func (state *State) narrowLocal(name string, value SymbolicValue, node parse.Node) bool {
	ok, _ := state.updateLocal2(name, node, func(expected SymbolicValue) (SymbolicValue, bool, error) {
		return value, false, nil
	}, true)
	return ok
}

func (state *State) updateLocal2(
	name string,
	node parse.Node,
	getValue func(expected SymbolicValue) (value SymbolicValue, deeperMismatch bool, _ error),
	narrowing bool,
) (bool, error) {
	state.assertHasLocals()
	scope := state.scopeStack[len(state.scopeStack)-1]
	if info, ok := scope.variables[name]; ok {
		value, deeperMismatch, err := getValue(info.static.SymbolicValue())
		if err != nil {
			return false, err
		}
		info.value = value

		if !isNever(value) {
			if !deeperMismatch && !info.static.TestValue(value) {
				msg := ""
				if narrowing {
					msg = fmtVarOfTypeCannotBeNarrowedToAn(info.static.SymbolicValue(), value)
				} else {
					msg = fmtNotAssignableToVarOftype(value, info.static)
				}
				state.addError(makeSymbolicEvalError(node, state, msg))
				return false, nil
			}
		}
		scope.variables[name] = info
		return true, nil
	}
	return false, nil
}

func (state *State) updateGlobal(name string, value SymbolicValue, node parse.Node) bool {
	ok, _ := state.updateGlobal2(name, node, func(expected SymbolicValue) (SymbolicValue, bool, error) {
		return value, false, nil
	}, false)
	return ok
}

func (state *State) narrowGlobal(name string, value SymbolicValue, node parse.Node) bool {
	ok, _ := state.updateGlobal2(name, node, func(expected SymbolicValue) (SymbolicValue, bool, error) {
		return value, false, nil
	}, true)
	return ok
}

func (state *State) updateGlobal2(
	name string,
	node parse.Node,
	getValue func(expected SymbolicValue) (value SymbolicValue, deeperMismatch bool, _ error),
	narrowing bool,
) (bool, error) {
	scope := state.scopeStack[0]
	if info, ok := scope.variables[name]; ok {
		value, deeperMismatch, err := getValue(info.static.SymbolicValue())
		if err != nil {
			return false, err
		}
		info.value = value

		if !isNever(value) {
			if !deeperMismatch && !info.static.TestValue(value) {
				msg := ""
				if narrowing {
					msg = fmtVarOfTypeCannotBeNarrowedToAn(info.static.SymbolicValue(), value)
				} else {
					msg = fmtNotAssignableToVarOftype(value, info.static)
				}
				state.addError(makeSymbolicEvalError(node, state, msg))
				return false, nil
			}
		}

		scope.variables[name] = info
		return true, nil
	}
	return false, nil
}

func (state *State) getStaticOfNode(partialNode parse.Node) (Pattern, bool) {
	//TODO: retrieve static from object

	switch node := partialNode.(type) {
	case *parse.Variable:
		info, ok := state.getLocal(node.Name)
		if !ok {
			return nil, false
		}
		return info.static, true
	case *parse.GlobalVariable:
		info, ok := state.getGlobal(node.Name)
		if !ok {
			return nil, false
		}
		return info.static, true
	case *parse.IdentifierLiteral:
		info, ok := state.get(node.Name)
		if !ok {
			return nil, false
		}
		return info.static, true
	case *parse.MemberExpression:
		leftStatic, _ := state.getStaticOfNode(node.Left)
		iprops, ok := leftStatic.(IPropsPattern)

		if !ok {
			return nil, false
		}

		propPattern, _, ok := iprops.ValuePropPattern(node.PropertyName.Name)
		if !ok {
			return nil, false
		}
		return propPattern, true
	case *parse.IdentifierMemberExpression:
		static, _ := state.getStaticOfNode(node.Left)

		for _, name := range node.PropertyNames {
			iprops, ok := static.(IPropsPattern)
			if !ok {
				return nil, false
			}

			propPattern, _, ok := iprops.ValuePropPattern(name.Name)
			if !ok {
				return nil, false
			}
			static = propPattern
		}

		return static, static != nil
	}

	return nil, false
}

func (state *State) pushScope() {
	state.scopeStack = append(state.scopeStack, &scopeInfo{
		variables: make(map[string]varSymbolicInfo),
	})
}

func (state *State) popScope() {
	state.scopeStack = state.scopeStack[:len(state.scopeStack)-1]
}

func (state *State) currentLocalScopeData() ScopeData {
	scope := state.scopeStack[len(state.scopeStack)-1]
	var vars []VarData
	for k, v := range scope.variables {
		vars = append(vars, VarData{
			Name:               k,
			Value:              v.value,
			DefinitionPosition: v.definitionPosition,
		})
	}
	return ScopeData{Variables: vars}
}

func (state *State) currentGlobalScopeData() ScopeData {
	scope := state.scopeStack[0]
	var vars []VarData
	for k, v := range scope.variables {
		vars = append(vars, VarData{
			Name:               k,
			Value:              v.value,
			DefinitionPosition: v.definitionPosition,
		})
	}
	return ScopeData{Variables: vars}
}

func (state *State) pushCallee(callNode *parse.CallExpression, callee *parse.FunctionExpression) bool {
	for _, c := range state.calleeStack {
		if callee == c {
			state.addError(makeSymbolicEvalError(callNode, state, FUNCS_CALLED_RECU_SHOULD_HAVE_RET_TYPE))
			return false
		}
	}
	state.calleeStack = append(state.calleeStack, callee)
	return true
}

func (state *State) popCallee() bool {
	state.calleeStack = state.calleeStack[:len(state.calleeStack)-1]
	return true
}

func (state *State) fork() *State {
	if len(state.scopeStack) == 0 { // 1 ?
		panic("cannot fork state with no local scope")
	}
	child := newSymbolicState(state.ctx.fork(), state.topChunk())
	child.ctx.associatedState = child
	child.parent = state
	child.Module = state.Module
	child.chunkStack = utils.CopySlice(state.chunkStack)
	child.symbolicData = state.symbolicData
	child.returnType = state.returnType
	child.baseGlobals = state.baseGlobals
	child.basePatterns = state.basePatterns
	child.basePatternNamespaces = state.basePatternNamespaces

	globalScopeCopy := &scopeInfo{
		variables: make(map[string]varSymbolicInfo, 0),
	}
	for k, v := range state.scopeStack[0].variables {
		globalScopeCopy.variables[k] = varSymbolicInfo{
			value:      v.value,
			static:     v.static,
			isConstant: v.isConstant,
		}
	}

	localScope := state.scopeStack[len(state.scopeStack)-1]
	localScopeCopy := &scopeInfo{
		variables: make(map[string]varSymbolicInfo, 0),
		self:      localScope.self,
	}
	for k, v := range localScope.variables {
		localScopeCopy.variables[k] = varSymbolicInfo{
			value:      v.value,
			static:     v.static,
			isConstant: v.isConstant,
		}
	}

	var calleeStackCopy []*parse.FunctionExpression
	copy(calleeStackCopy, state.calleeStack)
	child.calleeStack = calleeStackCopy

	child.scopeStack = []*scopeInfo{globalScopeCopy, localScopeCopy}

	return child
}

func (state *State) join(forks ...*State) {
	scope := state.scopeStack[len(state.scopeStack)-1]

	for _, fork := range forks {
		for k, forkVarInfo := range fork.scopeStack[len(fork.scopeStack)-1].variables {
			varInfo, ok := scope.variables[k]
			if !ok {
				continue
			}
			varInfo.value = joinValues([]SymbolicValue{varInfo.value, forkVarInfo.value})
			scope.variables[k] = varInfo
		}

		if fork.returnValue == nil {
			continue
		}

		if state.returnValue == nil {
			state.returnValue = fork.returnValue
			state.conditionalReturn = true
		} else {
			state.returnValue = joinValues([]SymbolicValue{state.returnValue, fork.returnValue})
		}
	}
}

func (state *State) addError(err SymbolicEvaluationError) {
	state.symbolicData.AddError(err)
}

func (state *State) addWarning(warning SymbolicEvaluationWarning) {
	state.symbolicData.AddWarning(warning)
}

func (state *State) addSymbolicGoFunctionError(msg string) {
	state.tempSymbolicGoFunctionErrors = append(state.tempSymbolicGoFunctionErrors, msg)
}

func (state *State) consumeSymbolicGoFunctionErrors(fn func(msg string)) {
	errors := state.tempSymbolicGoFunctionErrors
	for _, err := range errors {
		fn(err)
	}
	state.tempSymbolicGoFunctionErrors = state.tempSymbolicGoFunctionErrors[:0]
}

func (state *State) setSymbolicGoFunctionParameters(parameters *[]SymbolicValue, names []string) {
	if state.tempSymbolicGoFunctionParameters != nil {
		panic(errors.New("a temporary signature is already present"))
	}
	state.tempSymbolicGoFunctionParameterNames = names
	state.tempSymbolicGoFunctionParameters = parameters
}

func (state *State) consumeSymbolicGoFunctionParameters() ([]SymbolicValue, []string, bool) {
	if state.tempSymbolicGoFunctionParameters == nil {
		return nil, nil, false
	}
	defer func() {
		state.tempSymbolicGoFunctionParameters = nil
		state.tempSymbolicGoFunctionParameterNames = nil
	}()
	return *state.tempSymbolicGoFunctionParameters, state.tempSymbolicGoFunctionParameterNames, true
}

func (state *State) setUpdatedSelf(v SymbolicValue) {
	if state.tempUpdatedSelf != nil {
		panic(errors.New("an updated self is already present"))
	}
	state.tempUpdatedSelf = v
}

func (state *State) consumeUpdatedSelf() (SymbolicValue, bool) {
	defer func() {
		state.tempUpdatedSelf = nil
	}()
	if state.tempUpdatedSelf == nil {
		return nil, false
	}
	return state.tempUpdatedSelf, true
}

func (state *State) errors() []SymbolicEvaluationError {
	return state.symbolicData.errors
}
func (state *State) warnings() []SymbolicEvaluationWarning {
	return state.symbolicData.warnings
}

func (state *State) Errors() []SymbolicEvaluationError {
	return utils.CopySlice(state.symbolicData.errors)
}

func (state *State) Warnings() []SymbolicEvaluationWarning {
	return utils.CopySlice(state.symbolicData.warnings)
}

type varSymbolicInfo struct {
	value              SymbolicValue
	static             Pattern
	isConstant         bool
	definitionPosition parse.SourcePositionRange
}

func (info varSymbolicInfo) constness() GlobalConstness {
	if info.isConstant {
		return GlobalConst
	}
	return GlobalVar
}
