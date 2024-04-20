package symbolic

import (
	"errors"

	"github.com/go-git/go-billy/v5"
	"github.com/inoxlang/inox/internal/commonfmt"
	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/utils"
	"golang.org/x/exp/slices"
)

// State is the state of a symbolic evaluation.
// TODO: reduce memory usage of scopes
type State struct {
	parent *State //can be nil

	ctx        *Context
	chunkStack []*parse.ParsedChunkSource
	//positions of module/chunk import statements
	importPositions []parse.SourcePositionRange

	// first scope is the global scope, forks start with a global scope copy & a copy of the deepest local scope
	scopeStack            []*scopeInfo
	inPreinit             bool
	recursiveFunctionName string

	callStack    []inoxCallInfo
	topLevelSelf Value // can be nil

	returnType        Value
	returnValue       Value
	conditionalReturn bool

	yieldType        Value
	yieldedValue     Value
	conditionalYield bool

	iterationChange IterationChange

	checkMarkupInterpolation MarkupInterpolationCheckingFunction
	Module                   *Module

	//base globals and patterns

	baseGlobals           map[string]Value
	basePatterns          map[string]Pattern
	basePatternNamespaces map[string]*PatternNamespace

	//temporary fields storing information provided by symbolic Go functions during calls

	tempSymbolicGoFunctionErrors         []symbolicGoFunctionError
	tempSymbolicGoFunctionWarnings       []string
	tempSymbolicGoFunctionParameters     *[]Value
	tempSymbolicGoFunctionParameterNames []string
	tempSymbolicGoFunctionIsVariadic     bool
	tempUpdatedSelf                      Value

	lastErrorNode        parse.Node
	fmtHelper            *commonfmt.Helper
	symbolicData         *Data
	shellTrustedCommands []string

	testedProgram *TestedProgram //can be nil

	//nil if no project
	projectFilesystem billy.Filesystem
}

type IterationChange int

const (
	NoIterationChange IterationChange = iota
	BreakIteration
	ContinueIteration
	YieldItem
	PruneWalk
)

type GlobalConstness = int

const (
	GlobalVar GlobalConstness = iota
	GlobalConst
)

const (
	MAX_STRING_SUGGESTION_DIFF = 3
)

type ConcreteGlobalValue struct {
	Value      any
	IsConstant bool
}

func (v ConcreteGlobalValue) Constness() GlobalConstness {
	if v.IsConstant {
		return GlobalConst
	}
	return GlobalVar
}

type symbolicGoFunctionError struct {
	message  string
	location parse.Node //optional
}

type scopeInfo struct {
	self      Value //can be nil
	nextSelf  Value //can be nil
	variables map[string]varSymbolicInfo
}

type tempSymbolicGoFunctionSignature struct {
	params      []Value
	returnTypes []Value
}

func newSymbolicState(ctx *Context, chunk *parse.ParsedChunkSource) *State {
	chunkStack := []*parse.ParsedChunkSource{chunk}

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
		fmtHelper:       commonfmt.NewHelper(),
	}
	ctx.associatedState = state

	return state
}

func (state *State) getErrorMesssageLocation(node parse.Node) parse.SourcePositionStack {
	return state.getErrorMesssageLocationOfSpan(node.Base().Span)
}

func (state *State) getErrorMesssageLocationOfSpan(span parse.NodeSpan) parse.SourcePositionStack {
	sourcePositionStack := slices.Clone(state.importPositions)
	sourcePositionStack = append(sourcePositionStack, state.currentChunk().GetSourcePosition(span))
	return sourcePositionStack
}

func (state *State) topChunk() *parse.ParsedChunkSource {
	if len(state.chunkStack) == 0 {
		return nil
	}
	return state.chunkStack[0]
}

func (state *State) currentChunk() *parse.ParsedChunkSource {
	if len(state.chunkStack) == 0 {
		state.chunkStack = append(state.chunkStack, state.Module.mainChunk)
	}
	return state.chunkStack[len(state.chunkStack)-1]
}

func (state *State) pushChunk(chunk *parse.ParsedChunkSource, stmt *parse.InclusionImportStatement) {
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

func (state *State) setGlobal(name string, value Value, constness GlobalConstness, optDefinitionNode ...parse.Node) (ok bool) {
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

func (state *State) overrideGlobal(name string, value Value, optDefinitionNode ...parse.Node) (ok bool) {
	scope := state.scopeStack[0]
	info := scope.variables[name]
	info.value = value
	scope.variables[name] = info

	if len(optDefinitionNode) != 0 {
		info.definitionPosition = state.getCurrentChunkNodePositionOrZero(optDefinitionNode[0])
	}
	return true
}

func (state *State) setLocal(name string, value Value, static Pattern, optDefinitionNode ...parse.Node) {
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

func (state *State) overrideLocal(name string, value Value) {
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

func (state *State) setNextSelf(value Value) {
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

func (state *State) getNextSelf() (Value, bool) {
	state.assertHasLocals()
	scope := state.scopeStack[len(state.scopeStack)-1]

	return scope.nextSelf, scope.nextSelf != nil
}

func (state *State) setSelf(value Value) {
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

func (state *State) getSelf() (Value, bool) {
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

func (state *State) updateLocal(name string, value Value, node parse.Node) bool {
	ok, _ := state.updateLocal2(name, node, func(expected Value) (Value, bool, error) {
		return value, false, nil
	}, false)
	return ok
}

func (state *State) narrowLocal(name string, value Value, node parse.Node) bool {
	ok, _ := state.updateLocal2(name, node, func(expected Value) (Value, bool, error) {
		return value, false, nil
	}, true)
	return ok
}

func (state *State) updateLocal2(
	name string,
	node parse.Node,
	getValue func(expected Value) (value Value, deeperMismatch bool, _ error),
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
			if !deeperMismatch && !info.static.TestValue(value, RecTestCallState{}) {
				msg := ""
				var regions []commonfmt.RegionInfo
				if narrowing {
					msg, regions = fmtVarOfTypeCannotBeNarrowedToAn(state.fmtHelper, info.static.SymbolicValue(), value)
				} else {
					msg, regions = fmtNotAssignableToVarOftype(state.fmtHelper, value, info.static)
				}
				state.addError(MakeSymbolicEvalError(node, state, msg, regions...))
				return false, nil
			}
		}
		scope.variables[name] = info
		return true, nil
	}
	return false, nil
}

func (state *State) updateGlobal(name string, value Value, node parse.Node) bool {
	ok, _ := state.updateGlobal2(name, node, func(expected Value) (Value, bool, error) {
		return value, false, nil
	}, false)
	return ok
}

func (state *State) narrowGlobal(name string, value Value, node parse.Node) bool {
	ok, _ := state.updateGlobal2(name, node, func(expected Value) (Value, bool, error) {
		return value, false, nil
	}, true)
	return ok
}

func (state *State) updateGlobal2(
	name string,
	node parse.Node,
	getValue func(expected Value) (value Value, deeperMismatch bool, _ error),
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
			if !deeperMismatch && !info.static.TestValue(value, RecTestCallState{}) {
				msg := ""
				var regions []commonfmt.RegionInfo
				if narrowing {
					msg, regions = fmtVarOfTypeCannotBeNarrowedToAn(state.fmtHelper, info.static.SymbolicValue(), value)
				} else {
					msg, regions = fmtNotAssignableToVarOftype(state.fmtHelper, value, info.static)
				}
				state.addError(MakeSymbolicEvalError(node, state, msg, regions...))
				return false, nil
			}
		}

		scope.variables[name] = info
		return true, nil
	}
	return false, nil
}

func (state *State) getInfoOfNode(partialNode parse.Node) (static Pattern, ofConstantVar bool, ok bool) {
	//TODO: retrieve static from object

	switch node := partialNode.(type) {
	case *parse.Variable:
		info, ok := state.getGlobal(node.Name)
		if ok {
			return info.static, info.isConstant, true
		}

		info, ok = state.getLocal(node.Name)
		if !ok {
			return nil, false, false
		}
		return info.static, info.isConstant, true
	case *parse.IdentifierLiteral:
		info, ok := state.get(node.Name)
		if !ok {
			return nil, false, false
		}
		return info.static, info.isConstant, true
	case *parse.MemberExpression:
		leftStatic, ofConstant, _ := state.getInfoOfNode(node.Left)
		iprops, ok := leftStatic.(IPropsPattern)

		if !ok {
			return nil, false, false
		}

		propPattern, _, ok := iprops.ValuePropPattern(node.PropertyName.Name)
		if !ok {
			return nil, false, false
		}
		return propPattern, ofConstant, true
	case *parse.DoubleColonExpression:
		leftStatic, ofConstant, _ := state.getInfoOfNode(node.Left)
		iprops, ok := leftStatic.(IPropsPattern)

		if !ok {
			return nil, false, false
		}

		propPattern, _, ok := iprops.ValuePropPattern(node.Element.Name)
		if !ok {
			return nil, false, false
		}
		return propPattern, ofConstant, true
	case *parse.IdentifierMemberExpression:
		static, ofConstant, _ := state.getInfoOfNode(node.Left)

		for _, name := range node.PropertyNames {
			iprops, ok := static.(IPropsPattern)
			if !ok {
				return nil, false, false
			}

			propPattern, _, ok := iprops.ValuePropPattern(name.Name)
			if !ok {
				return nil, false, false
			}
			static = propPattern
		}

		return static, ofConstant, static != nil
	}

	return nil, false, false
}

func (state *State) getStaticOfNode(partialNode parse.Node) (Pattern, bool) {
	static, _, ok := state.getInfoOfNode(partialNode)
	return static, ok
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
	return ScopeData{Variables: vars, Chunk: state.currentChunk().Node}
}

func (state *State) pushInoxCall(call inoxCallInfo) bool {
	for _, c := range state.callStack {
		if call.calleeFnExpr == c.calleeFnExpr {
			state.addError(MakeSymbolicEvalError(call.callNode, state, FUNCS_CALLED_RECU_SHOULD_HAVE_RET_TYPE))
			return false
		}
	}
	state.callStack = append(state.callStack, call)
	return true
}

func (state *State) popCall() bool {
	state.callStack = state.callStack[:len(state.callStack)-1]
	return true
}

func (state *State) currentInoxCall() (inoxCallInfo, bool) {
	if len(state.callStack) > 0 {
		return state.callStack[len(state.callStack)-1], true
	}
	return inoxCallInfo{}, false
}

func (state *State) inNonInitialInoxCall() bool {
	call, yes := state.currentInoxCall()
	return yes && !call.isInitialCheckCall
}

func (state *State) fork() *State {
	if len(state.scopeStack) == 0 { // 1 ?
		panic("cannot fork state with no local scope")
	}
	child := newSymbolicState(state.ctx.fork(), state.topChunk())
	child.ctx.associatedState = child
	child.parent = state
	child.Module = state.Module
	child.chunkStack = slices.Clone(state.chunkStack)
	child.symbolicData = state.symbolicData
	child.shellTrustedCommands = state.shellTrustedCommands
	child.returnType = state.returnType
	child.baseGlobals = state.baseGlobals
	child.basePatterns = state.basePatterns
	child.basePatternNamespaces = state.basePatternNamespaces
	child.checkMarkupInterpolation = state.checkMarkupInterpolation
	child.projectFilesystem = state.projectFilesystem

	globalScopeCopy := &scopeInfo{
		variables: make(map[string]varSymbolicInfo, 0),
	}
	for k, v := range state.scopeStack[0].variables {
		globalScopeCopy.variables[k] = varSymbolicInfo{
			value:              v.value,
			static:             v.static,
			isConstant:         v.isConstant,
			definitionPosition: v.definitionPosition,
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

	var callStackCopy []inoxCallInfo
	copy(callStackCopy, state.callStack)
	child.callStack = callStackCopy

	child.scopeStack = []*scopeInfo{globalScopeCopy, localScopeCopy}

	return child
}

func (state *State) join(areAllOutcomesCovered bool, forks ...*State) {
	scope := state.scopeStack[len(state.scopeStack)-1]

	var varsUpdatedByAllForks []string

	if areAllOutcomesCovered {
		//Determine the list of variables updated by all forks.

		for varName, varInfo := range scope.variables {

			updatedByAllForks := true

			for _, fork := range forks {
				forkVarInfo, ok := fork.scopeStack[len(fork.scopeStack)-1].variables[varName]

				if !ok {
					panic(ErrUnreachable)
				}

				if forkVarInfo.value == varInfo.value {
					updatedByAllForks = false
					break
				}
			}

			if updatedByAllForks {
				varsUpdatedByAllForks = append(varsUpdatedByAllForks, varName)
			}
		}
	}

	atLeastOneForkReturns := utils.Some(forks, func(fork *State) bool {
		return fork.returnValue != nil
	})

	doAllForksReturn := false

	if atLeastOneForkReturns {
		doAllForksReturn = true

		for _, fork := range forks {
			if fork.returnValue == nil {
				doAllForksReturn = false
				break
			}
		}
	}

	atLeastOneForkYields := utils.Some(forks, func(fork *State) bool {
		return fork.yieldedValue != nil
	})

	doAllForksYield := false

	if atLeastOneForkYields {
		doAllForksYield = true

		for _, fork := range forks {
			if fork.yieldedValue == nil {
				doAllForksYield = false
				break
			}
		}
	}

	for _, fork := range forks {
		for varName, forkVarInfo := range fork.scopeStack[len(fork.scopeStack)-1].variables {

			varInfo, ok := scope.variables[varName]
			if !ok {
				//The variable is only present in the fork.
				continue
			}

			if index := slices.Index(varsUpdatedByAllForks, varName); index >= 0 {
				//Since the variable is updated by all forks we can ignore the original value.
				varInfo.value = forkVarInfo.value

				//Remove the variable from the list so that we can join the current value with values from other forks.
				varsUpdatedByAllForks = slices.Delete(varsUpdatedByAllForks, index, index+1)
			} else {
				varInfo.value = joinValues([]Value{varInfo.value, forkVarInfo.value})
			}

			scope.variables[varName] = varInfo
		}

		if fork.returnValue != nil {
			//Join the value returned by the state with the value returned by the fork.
			if state.returnValue == nil {
				state.returnValue = fork.returnValue
				state.conditionalReturn = true
			} else {
				state.returnValue = joinValues([]Value{state.returnValue, fork.returnValue})
			}
		}

		if fork.yieldedValue != nil {
			//Join the value yielded by the state with the value yielded by the fork.
			if state.yieldedValue == nil {
				state.yieldedValue = fork.yieldedValue
				state.conditionalYield = true
			} else {
				state.yieldedValue = joinValues([]Value{state.yieldedValue, fork.yieldedValue})
			}
		}
	}

	if areAllOutcomesCovered && doAllForksReturn {
		state.conditionalReturn = false
	}

	if areAllOutcomesCovered && doAllForksYield {
		state.conditionalYield = false
	}
}

func (state *State) addError(err SymbolicEvaluationError) {
	state.symbolicData.AddError(err)
}

func (state *State) addErrorIf(cond bool, err SymbolicEvaluationError) {
	if cond {
		state.symbolicData.AddError(err)
	}
}

func (state *State) addWarning(warning SymbolicEvaluationWarning) {
	state.symbolicData.AddWarning(warning)
}

func (state *State) addSymbolicGoFunctionError(msg string) {
	state.tempSymbolicGoFunctionErrors = append(state.tempSymbolicGoFunctionErrors, symbolicGoFunctionError{
		message: msg,
	})
}

func (state *State) addLocatedSymbolicGoFunctionError(msg string, location parse.Node) {
	state.tempSymbolicGoFunctionErrors = append(state.tempSymbolicGoFunctionErrors, symbolicGoFunctionError{
		message:  msg,
		location: location,
	})
}

func (state *State) consumeSymbolicGoFunctionErrors(fn func(msg string, optionalLocaction parse.Node)) {
	errors := state.tempSymbolicGoFunctionErrors
	for _, err := range errors {
		fn(err.message, err.location)
	}
	state.tempSymbolicGoFunctionErrors = state.tempSymbolicGoFunctionErrors[:0]
}

func (state *State) addSymbolicGoFunctionWarning(msg string) {
	state.tempSymbolicGoFunctionWarnings = append(state.tempSymbolicGoFunctionWarnings, msg)
}

func (state *State) consumeSymbolicGoFunctionWarnings(fn func(msg string)) {
	warnings := state.tempSymbolicGoFunctionWarnings
	for _, warning := range warnings {
		fn(warning)
	}
	state.tempSymbolicGoFunctionWarnings = state.tempSymbolicGoFunctionWarnings[:0]
}

func (state *State) setSymbolicGoFunctionParameters(parameters *[]Value, names []string, isVariadic bool) {
	if state.tempSymbolicGoFunctionParameters != nil {
		panic(errors.New("a temporary signature is already present"))
	}
	state.tempSymbolicGoFunctionParameterNames = names
	state.tempSymbolicGoFunctionParameters = parameters
	state.tempSymbolicGoFunctionIsVariadic = isVariadic
}

func (state *State) consumeSymbolicGoFunctionParameters() (paramTypes []Value, paramNames []string, variadic bool, present bool) {
	if state.tempSymbolicGoFunctionParameters == nil {
		return nil, nil, false, false
	}
	defer func() {
		state.tempSymbolicGoFunctionParameters = nil
		state.tempSymbolicGoFunctionParameterNames = nil
		state.tempSymbolicGoFunctionIsVariadic = false
	}()
	return *state.tempSymbolicGoFunctionParameters, state.tempSymbolicGoFunctionParameterNames, state.tempSymbolicGoFunctionIsVariadic, true
}

func (state *State) resetGoFunctionRelatedFields() {
	state.tempSymbolicGoFunctionErrors = state.tempSymbolicGoFunctionErrors[:0]
	state.tempSymbolicGoFunctionWarnings = state.tempSymbolicGoFunctionWarnings[:0]
	state.tempSymbolicGoFunctionParameters = nil
	state.tempSymbolicGoFunctionParameterNames = nil
	state.tempSymbolicGoFunctionIsVariadic = false
}

func (state *State) setUpdatedSelf(v Value) {
	if state.tempUpdatedSelf != nil {
		panic(errors.New("an updated self is already present"))
	}
	state.tempUpdatedSelf = v
}

func (state *State) consumeUpdatedSelf() (Value, bool) {
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
	return slices.Clone(state.symbolicData.errors)
}

func (state *State) Warnings() []SymbolicEvaluationWarning {
	return slices.Clone(state.symbolicData.warnings)
}

func (state *State) FormatHelper() *commonfmt.Helper {
	return state.FormatHelper()
}

type varSymbolicInfo struct {
	value              Value
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
