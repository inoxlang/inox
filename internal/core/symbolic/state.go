package internal

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

	// first scope is the global scope, forks start with a global scope copy & a copy of the deepest local scope
	scopeStack []*scopeInfo

	calleeStack     []*parse.FunctionExpression
	topLevelSelf    SymbolicValue // can be nil
	returnType      *SymbolicValue
	returnValue     *SymbolicValue
	iterationChange IterationChange
	Module          *Module

	tempSymbolicGoFunctionErrors []string
	errors                       []SymbolicEvaluationError
	errorMessageSet              map[string]bool
	symbolicData                 *SymbolicData
}

type scopeInfo struct {
	self      SymbolicValue //can be nil
	nextSelf  SymbolicValue //can be nil
	variables map[string]varSymbolicInfo
}

func newSymbolicState(ctx *Context, optionalChunk *parse.ParsedChunk) *State {
	var chunkStack []*parse.ParsedChunk
	if optionalChunk != nil {
		chunkStack = append(chunkStack, optionalChunk)
	}
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
		errorMessageSet: map[string]bool{},
	}
	ctx.associatedState = state

	return state
}

func (state *State) getErrorMesssageLocation(node parse.Node) parse.SourcePositionStack {
	var sourcePositionStack parse.SourcePositionStack
	for _, chunk := range state.chunkStack {
		sourcePositionStack = append(sourcePositionStack, chunk.GetSourcePosition(node.Base().Span))
	}
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
		state.chunkStack = append(state.chunkStack, state.Module.MainChunk)
	}
	return state.chunkStack[len(state.chunkStack)-1]
}

func (state *State) pushChunk(chunk *parse.ParsedChunk) {
	state.chunkStack = append(state.chunkStack, chunk)
}

func (state *State) popChunk() {
	state.chunkStack = state.chunkStack[:len(state.chunkStack)-1]
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

func (state *State) setGlobal(name string, value SymbolicValue, constness GlobalConstness) (ok bool) {
	scope := state.scopeStack[0]
	var info varSymbolicInfo

	if info_, alreadyDefined := scope.variables[name]; alreadyDefined {
		info = info_
		info.value = value
	} else {
		info = varSymbolicInfo{
			isConstant: constness == GlobalConst,
			static:     &TypePattern{val: value.WidestOfType()},
			value:      value,
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

func (state *State) setLocal(name string, value SymbolicValue, static Pattern) {
	state.assertHasLocals()
	scope := state.scopeStack[len(state.scopeStack)-1]

	if static == nil {
		static = &TypePattern{val: value.WidestOfType()}
	}

	scope.variables[name] = varSymbolicInfo{
		value:  value,
		static: static,
	}
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
	state.assertHasLocals()
	scope := state.scopeStack[len(state.scopeStack)-1]
	if info, ok := scope.variables[name]; ok {
		info.value = value

		widenedValue := value

		for !isAny(widenedValue) && !info.static.TestValue(widenedValue) {
			widenedValue = widenOrAny(widenedValue)
		}

		if !info.static.TestValue(widenedValue) {
			state.addError(makeSymbolicEvalError(node, state, fmtNotAssignableToVarOftype(value, info.static)))
			return false
		}
		scope.variables[name] = info
		return true
	}
	return false
}

func (state *State) updateGlobal(name string, value SymbolicValue, node parse.Node) bool {
	scope := state.scopeStack[0]
	if info, ok := scope.variables[name]; ok {
		info.value = value

		widenedValue := value

		for !isAny(widenedValue) && !info.static.TestValue(widenedValue) {
			widenedValue = widenOrAny(widenedValue)
		}

		if !info.static.TestValue(widenedValue) {
			state.addError(makeSymbolicEvalError(node, state, fmtNotAssignableToVarOftype(value, info.static)))
			return false
		}
		scope.variables[name] = info
		return true
	}
	return false
}

func (state *State) updateVar(name string, value SymbolicValue, node parse.Node) bool {
	if state.hasGlobal(name) {
		return state.updateGlobal(name, value, node)
	}
	return state.updateLocal(name, value, node)
}

func (state *State) deleteLocal(name string) {
	state.assertHasLocals()
	scope := state.scopeStack[len(state.scopeStack)-1]

	delete(scope.variables, name)
}

func (state *State) pushScope() {
	state.scopeStack = append(state.scopeStack, &scopeInfo{
		variables: make(map[string]varSymbolicInfo),
	})
}

func (state *State) popScope() {
	state.scopeStack = state.scopeStack[:len(state.scopeStack)-1]
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
	}
}

func (state *State) addError(err SymbolicEvaluationError) {
	if state.errorMessageSet[err.Error()] {
		return
	}
	state.errorMessageSet[err.Error()] = true

	state.errors = append(state.errors, err)
	if state.parent != nil {
		state.parent.addError(err)
	}
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

type varSymbolicInfo struct {
	value      SymbolicValue
	static     Pattern
	isConstant bool
}

func (info varSymbolicInfo) constness() GlobalConstness {
	if info.isConstant {
		return GlobalConst
	}
	return GlobalVar
}
