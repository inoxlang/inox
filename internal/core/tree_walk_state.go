package core

import (
	"errors"

	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/parse"
)

// A TreeWalkState stores all the state necessary to perform a tree walking evaluation.
type TreeWalkState struct {
	Global                            *GlobalState
	LocalScopeStack                   []map[string]Value //TODO: reduce memory usage by using a struct { small *memds.Map8[string,Value]; grown map[string]Value } ?
	frameInfo                         []StackFrameInfo   //used for debugging only, the list is reversed
	chunkStack                        []*parse.ChunkStackItem
	earlyFunctionDeclarationsPosition int32 //-1 if no position, specific to the current chunk.
	earlyFunctionDeclarations         []*parse.FunctionDeclaration
	fullChunkStack                    []*parse.ChunkStackItem //chunk stack but including calls' chunks

	constantVars map[string]bool
	postHandle   func(node parse.Node, val Value, err error) (Value, error)

	debug           *Debugger
	returnValue     Value           //return value from a function or module
	iterationChange IterationChange //break, continue, prune
	self            Value           //value of self in methods
	entryComputeFn  func(v Value) (Value, error)

	forceDisableTesting bool //used to disable testing in included chunks

	comptimeTypes *ModuleComptimeTypes

	//Fields added in the future should be reset in Reset().
}

// NewTreeWalkState creates a TreeWalkState and a GlobalState it will use.
func NewTreeWalkState(ctx *Context, constants ...map[string]Value) *TreeWalkState {
	global := NewGlobalState(ctx, constants...)

	return NewTreeWalkStateWithGlobal(global)
}

// NewTreeWalkState creates a TreeWalkState that will use $global as its global state.
func NewTreeWalkStateWithGlobal(global *GlobalState) *TreeWalkState {

	state := &TreeWalkState{
		LocalScopeStack:                   []map[string]Value{},
		constantVars:                      make(map[string]bool, 0),
		Global:                            global,
		earlyFunctionDeclarationsPosition: -1,
	}

	if global.Module != nil {
		chunk := &parse.ChunkStackItem{
			Chunk: global.Module.MainChunk,
		}
		state.chunkStack = append(state.chunkStack, chunk)
		state.fullChunkStack = append(state.fullChunkStack, chunk)
	}

	return state
}

// Reset recycles the state by resetting its fields. Since references to the state may exist somewhere
// Reset() should only be used for very simple programs, at least for now. Calling Reset() with a *GlobalState
// is almost equivalent as creating a new state with NewTreeWalkStateWithGlobal. It is recommended to call
// Reset(nil) if the caller is able to know when the previous module has finished executing: this will remove
// references to the old state and to some values.
func (state *TreeWalkState) Reset(global *GlobalState) {
	if !state.Global.Ctx.IsDone() {
		panic(errors.New("cannot reset a tree-walk state that is still in use"))
	}

	state.Global = global
	state.LocalScopeStack = state.LocalScopeStack[:0]
	state.chunkStack = state.chunkStack[:0]
	state.fullChunkStack = state.fullChunkStack[:0]
	state.earlyFunctionDeclarationsPosition = -1
	state.earlyFunctionDeclarations = nil

	if global != nil {
		chunk := &parse.ChunkStackItem{Chunk: global.Module.MainChunk}
		state.chunkStack = append(state.chunkStack, chunk)
		state.fullChunkStack = append(state.fullChunkStack, chunk)
	}

	clear(state.constantVars)
	state.iterationChange = NoIterationChange

	state.returnValue = nil
	state.self = nil
	state.comptimeTypes = nil
	state.entryComputeFn = nil

	state.forceDisableTesting = false
	state.postHandle = nil
	state.debug = nil
	state.frameInfo = state.frameInfo[:0]
}

func (state TreeWalkState) currentChunkStackItem() *parse.ChunkStackItem {
	if len(state.chunkStack) == 0 {
		state.chunkStack = append(state.chunkStack, &parse.ChunkStackItem{
			Chunk: state.Global.Module.MainChunk,
		})
	}
	return state.chunkStack[len(state.chunkStack)-1]
}

func (state TreeWalkState) currentChunk() *parse.ParsedChunkSource {
	return state.currentFullChunkStackItem().Chunk
}

func (state TreeWalkState) currentFullChunkStackItem() *parse.ChunkStackItem {
	if len(state.fullChunkStack) == 0 {
		state.fullChunkStack = append(state.fullChunkStack, state.currentChunkStackItem())
	}
	return state.fullChunkStack[len(state.fullChunkStack)-1]
}

func (state *TreeWalkState) pushImportedChunk(chunk *parse.ParsedChunkSource, importNode *parse.InclusionImportStatement) {
	state.currentFullChunkStackItem().CurrentNodeSpan = importNode.Span
	pushedChunk := &parse.ChunkStackItem{
		Chunk: chunk,
	}
	state.chunkStack = append(state.chunkStack, pushedChunk)
	state.fullChunkStack = append(state.fullChunkStack, pushedChunk)
}

func (state *TreeWalkState) popImportedChunk() {
	state.chunkStack = state.chunkStack[:len(state.chunkStack)-1]
	state.fullChunkStack = state.fullChunkStack[:len(state.fullChunkStack)-1]

	if len(state.fullChunkStack) != 0 {
		state.currentFullChunkStackItem().CurrentNodeSpan = parse.NodeSpan{}
	}
}

func (state *TreeWalkState) pushChunkOfCall(chunk *parse.ParsedChunkSource, callingNode parse.Node) {
	state.currentFullChunkStackItem().CurrentNodeSpan = callingNode.Base().Span
	pushedChunk := &parse.ChunkStackItem{
		Chunk: chunk,
	}
	state.fullChunkStack = append(state.fullChunkStack, pushedChunk)
}

func (state *TreeWalkState) popChunkOfCall() {
	state.fullChunkStack = state.fullChunkStack[:len(state.fullChunkStack)-1]
	if len(state.fullChunkStack) != 0 {
		state.currentFullChunkStackItem().CurrentNodeSpan = parse.NodeSpan{}
	}
}

func (state *TreeWalkState) SetGlobal(name string, value Value, constness GlobalConstness) (ok bool) {
	if state.constantVars[name] {
		return false
	}

	state.Global.Globals.Set(name, value)

	if constness == GlobalConst {
		state.constantVars[name] = true
	}

	if watchable, ok := value.(SystemGraphNodeValue); ok {
		state.Global.ProposeSystemGraph(watchable, name)
	}

	return true
}

func (state *TreeWalkState) HasGlobal(name string) bool {
	return state.Global.Globals.Has(name)
}

func (state *TreeWalkState) Get(name string) (Value, bool) {
	for i := len(state.LocalScopeStack) - 1; i >= 0; i-- {
		if v, ok := state.LocalScopeStack[i][name]; ok {
			return v, true
		}
	}
	val := state.Global.Globals.Get(name)
	return val, val != nil
}

func (state *TreeWalkState) GetGlobal(name string) (Value, bool) {
	val := state.Global.Globals.Get(name)
	return val, val != nil
}

func (state *TreeWalkState) CurrentLocalScope() map[string]Value {
	if len(state.LocalScopeStack) == 0 {
		return nil
	}
	return state.LocalScopeStack[len(state.LocalScopeStack)-1]
}

func (state *TreeWalkState) PushScope() {
	state.LocalScopeStack = append(state.LocalScopeStack, make(map[string]Value))
}

func (state *TreeWalkState) PopScope() {
	state.LocalScopeStack = state.LocalScopeStack[:len(state.LocalScopeStack)-1]
}

func (state *TreeWalkState) GetGlobalState() *GlobalState {
	return state.Global
}

func (state *TreeWalkState) AttachDebugger(debugger *Debugger) {
	if state.debug != nil {
		panic(ErrDebuggerAlreadyAttached)
	}

	if !state.Global.Debugger.CompareAndSwap(nil, debugger) {
		panic(ErrDebuggerAlreadyAttached)
	}

	state.debug = debugger
}

func (state *TreeWalkState) DetachDebugger() {
	state.debug = nil
	state.Global.Debugger.Store((*Debugger)(nil))
}

func (state *TreeWalkState) updateStackTrace(currentStmt parse.Node) {
	currentFrame := state.frameInfo[len(state.frameInfo)-1]
	currentFrame.Node = currentStmt

	line, col := currentFrame.Chunk.GetLineColumn(currentStmt)
	currentFrame.StatementStartLine = line
	currentFrame.StatementStartColumn = col

	state.frameInfo[len(state.frameInfo)-1] = currentFrame
}

func (state *TreeWalkState) formatLocation(node parse.Node) (parse.SourcePositionStack, string) {
	return parse.GetSourcePositionStack(node.Base().Span, state.fullChunkStack)
}

func (state *TreeWalkState) getConcreteType(symbolic symbolic.CompileTimeType) CompileTimeType {
	if state.comptimeTypes == nil {
		types, _ := state.Global.SymbolicData.GetComptimeTypes(state.Global.Module.MainChunk.Node)
		state.comptimeTypes = NewModuleComptimeTypes(types /*ok even if nil*/)
	}
	return state.comptimeTypes.getConcreteType(symbolic)
}

type IterationChange int

const (
	NoIterationChange IterationChange = iota
	BreakIteration
	ContinueIteration
	PruneWalk
)

type GlobalConstness = int

const (
	GlobalVar GlobalConstness = iota
	GlobalConst
)
