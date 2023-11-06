package core

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/inoxlang/inox/internal/core/symbolic"
	parse "github.com/inoxlang/inox/internal/parse"
	permkind "github.com/inoxlang/inox/internal/permkind"
	"github.com/inoxlang/inox/internal/utils"
)

const (
	ROUTINE_POST_YIELD_PAUSE = time.Microsecond
)

var (
	ROUTINE_PROPNAMES       = []string{"wait_result", "cancel", "steps"}
	ROUTINE_GROUP_PROPNAMES = []string{"wait_results", "cancel_all"}
	EXECUTED_STEP_PROPNAMES = []string{"result", "end_time"}
)

func init() {
	RegisterSymbolicGoFunction(NewLThreadGroup, func(xtx *symbolic.Context) *symbolic.LThreadGroup {
		return &symbolic.LThreadGroup{}
	})
}

// A LThread is similar to a goroutine in Golang, it represents the execution of a single module and can be cancelled at any time.
type LThread struct {
	useBytecode bool
	module      *Module
	state       *GlobalState
	lock        sync.Mutex

	//steps
	executedSteps          []*ExecutedStep
	executedStepCallbackFn func(step ExecutedStep, lthread *LThread) (continueExec bool)
	continueExecChan       chan struct{}
	paused                 atomic.Bool

	//result
	result      Value
	err         Error
	done        atomic.Bool
	wait_result chan struct{}
}

type LthreadSpawnArgs struct {
	SpawnerState *GlobalState
	Globals      GlobalVariables
	NewGlobals   []string //listed globals are not shared nor cloned.

	Module       *Module
	Manifest     *Manifest
	LthreadCtx   *Context
	PreinitState *GlobalState

	IsTestingEnabled bool
	TestFilters      TestFilters
	TestItem         TestItem
	TestedProgram    *Module

	//AbsScriptDir string
	Bytecode    *Bytecode
	UseBytecode bool
	StartPaused bool
	Self        Value
	Timeout     time.Duration

	// Even if true a token is taken for the threads/simul-instances limit
	IgnoreCreateLThreadPermCheck bool
	PauseAfterYield              bool
}

// SpawnLThread spawns a new lthread, if .LthreadCtx is nil a minimal context is created for the lthread.
// The provided globals that are not thread safe are wrapped in a SharedValue.
func SpawnLThread(args LthreadSpawnArgs) (*LThread, error) {

	if !args.IgnoreCreateLThreadPermCheck {
		perm := LThreadPermission{Kind_: permkind.Create}

		if err := args.SpawnerState.Ctx.CheckHasPermission(perm); err != nil {
			return nil, fmt.Errorf("cannot spawn lthread: %s", err.Error())
		}
	}

	if args.LthreadCtx == nil {
		args.LthreadCtx = NewContext(ContextConfig{
			Permissions: []Permission{
				GlobalVarPermission{Kind_: permkind.Read, Name: "*"},
				GlobalVarPermission{Kind_: permkind.Use, Name: "*"},
				GlobalVarPermission{Kind_: permkind.Create, Name: "*"},
			},
			ParentContext: args.SpawnerState.Ctx,
		})
	}

	staticCheckData, err := StaticCheck(StaticCheckInput{
		State:             args.SpawnerState,
		Node:              args.Module.MainChunk.Node,
		Module:            args.Module,
		Chunk:             args.Module.MainChunk,
		Globals:           args.Globals,
		Patterns:          args.LthreadCtx.namedPatterns,
		PatternNamespaces: args.LthreadCtx.patternNamespaces,
	})
	if err != nil {
		return nil, fmt.Errorf("cannot spawn lthread: expression: module/expr checking failed: %w", err)
	}

	//set global variables & constants
	modState := NewGlobalState(args.LthreadCtx, args.Globals.Constants())
	err = args.Globals.Foreach(func(name string, v Value, isConstant bool) error {
		if isConstant {
			return nil
		}

		if slices.Contains(args.NewGlobals, name) {
			modState.Globals.Set(name, v)
		} else {
			shared, err := ShareOrClone(v, args.SpawnerState)
			if err != nil {
				return fmt.Errorf("failed to share/clone provided global '%s': %w", name, err)
			}
			modState.Globals.Set(name, shared)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	modState.Module = args.Module
	modState.Manifest = args.Manifest
	modState.MainState = args.SpawnerState.MainState
	modState.Bytecode = args.Bytecode
	modState.Logger = args.SpawnerState.Logger
	modState.Out = args.SpawnerState.Out
	modState.StaticCheckData = staticCheckData
	modState.GetBaseGlobalsForImportedModule = args.SpawnerState.GetBaseGlobalsForImportedModule
	modState.GetBasePatternsForImportedModule = args.SpawnerState.GetBasePatternsForImportedModule
	// TODO: set SymbolicData
	if args.IsTestingEnabled {
		modState.IsTestingEnabled = true
		modState.TestFilters = args.TestFilters

		if args.TestItem != nil {
			modState.TestItem = args.TestItem
			modState.TestedProgram = args.TestedProgram
			modState.TestItemFullName = makeTestFullName(args.TestItem, args.SpawnerState)
		}
	}
	modState.OutputFieldsInitialized.Store(true)

	return SpawnLthreadWithState(LthreadWithStateSpawnArgs{
		Timeout:         args.Timeout,
		SpawnerState:    args.SpawnerState,
		State:           modState,
		UseBytecode:     args.UseBytecode,
		PauseAfterYield: args.PauseAfterYield,
		StartPaused:     args.StartPaused,
		Self:            args.Self,
	})
}

type LthreadWithStateSpawnArgs struct {
	Timeout         time.Duration
	SpawnerState    *GlobalState
	State           *GlobalState
	UseBytecode     bool
	PauseAfterYield bool
	StartPaused     bool

	Self Value
}

func SpawnLthreadWithState(args LthreadWithStateSpawnArgs) (*LThread, error) {
	modState := args.State

	lthread := &LThread{
		module:           modState.Module,
		state:            modState,
		wait_result:      make(chan struct{}, 1),
		continueExecChan: make(chan struct{}, 1),
		useBytecode:      args.UseBytecode,
		executedStepCallbackFn: func(step ExecutedStep, lthread *LThread) (continueExec bool) {
			return !args.PauseAfterYield
		},
	}
	modState.LThread = lthread

	if args.Timeout != 0 {
		go func(d time.Duration) {
			<-time.After(d)
			modState.Ctx.CancelGracefully()
		}(args.Timeout)
	}

	if args.StartPaused {
		lthread.paused.Store(true)
	}

	if err := args.SpawnerState.Ctx.Take(THREADS_SIMULTANEOUS_INSTANCES_LIMIT_NAME, 1); err != nil {
		return nil, fmt.Errorf("cannot spawn lthread: %s", err.Error())
	}

	// goroutine in which the lthread's module is evaluated
	go func(modState *GlobalState, chunk parse.Node, lthread *LThread, startPaused bool, self Value) {
		var res Value
		var err error

		defer func() {
			e := recover()
			if v, ok := e.(error); ok {
				err = v
			}

			lthread.lock.Lock()
			defer lthread.lock.Unlock()

			close(lthread.continueExecChan)
			lthread.done.Store(true)
			lthread.paused.Store(false)

			if res == nil {
				res = Nil
			}

			if err == nil {
				res, err = ShareOrClone(res, lthread.state)
			}

			if err != nil {
				modState.Logger.Print("a lthread failed or was cancelled: " + utils.AddCarriageReturnAfterNewlines(err.Error()))
				lthread.result = Nil
				lthread.err = ValOf(err).(Error)
				lthread.wait_result <- struct{}{}
				return
			}

			lthread.result = res
			lthread.wait_result <- struct{}{}
		}()

		defer func() {
			args.SpawnerState.Ctx.GiveBack(THREADS_SIMULTANEOUS_INSTANCES_LIMIT_NAME, 1)
		}()

		if startPaused {
			select {
			case <-lthread.continueExecChan:
				//
			case <-modState.Ctx.Done():
				panic(context.Canceled)
			}
		}

		defer modState.Ctx.CancelGracefully()
		defer modState.Ctx.DefinitelyStopCPUDecrementation()

		if args.UseBytecode {
			res, err = EvalBytecode(lthread.state.Bytecode, modState, args.Self)
		} else {
			state := NewTreeWalkStateWithGlobal(modState)
			state.self = args.Self

			parentDebugger, ok := args.SpawnerState.Debugger.Load().(*Debugger)
			if ok && !parentDebugger.Closed() {
				debugger := parentDebugger.NewChild()

				parentDebugger.ControlChan() <- DebugCommandInformAboutSecondaryEvent{
					Event: LThreadSpawnedEvent{
						StateId: modState.id,
					},
				}
				debugger.AttachAndStart(state)
				modState.Debugger.Store(debugger)

				defer func() {
					debugger.ControlChan() <- DebugCommandCloseDebugger{}
				}()
			}

			res, err = TreeWalkEval(chunk, state)
		}

	}(modState, modState.Module.MainChunk.Node, lthread, args.StartPaused, args.Self)

	return lthread, nil
}

func (r *LThread) GetGoMethod(name string) (*GoFunction, bool) {
	switch name {
	case "wait_result":
		return WrapGoMethod(r.WaitResult), true
	case "cancel":
		return WrapGoMethod(r.Cancel), true
	}
	return nil, false
}

func (r *LThread) Prop(ctx *Context, name string) Value {
	switch name {
	case "steps":
		r.lock.Lock()
		defer r.lock.Unlock()
		steps := make([]Value, len(r.executedSteps))

		for i := 0; i < len(r.executedSteps); i++ {
			steps[i] = r.executedSteps[i]
		}
		return NewArrayFrom(steps...)
	}
	method, ok := r.GetGoMethod(name)
	if !ok {
		panic(FormatErrPropertyDoesNotExist(name, r))
	}
	return method
}

func (*LThread) SetProp(ctx *Context, name string, value Value) error {
	return ErrCannotSetProp
}

func (*LThread) PropertyNames(ctx *Context) []string {
	return ROUTINE_PROPNAMES
}

// Cancel stops the execution of the lthread.
func (lthread *LThread) IsDone() bool {
	return lthread.done.Load()
}

// yield creates a new ExecutedStep with the given result, if a the step callback function is set it is executed
// to determinate if execution will be paused, if not set the execution is paused.
func (lthread *LThread) yield(ctx *Context, value Value) {
	if lthread.IsDone() {
		panic(ErrLThreadIsDone)
	}

	lthread.lock.Lock()
	unlock := true
	defer func() {
		if unlock {
			lthread.lock.Unlock()
		}
	}()

	step := &ExecutedStep{
		result:  value,
		endTime: time.Now(),
	}

	lthread.executedSteps = append(lthread.executedSteps, step)

	if lthread.executedStepCallbackFn != nil {
		if continueExec := lthread.executedStepCallbackFn(*step, lthread); continueExec {
			time.Sleep(ROUTINE_POST_YIELD_PAUSE)
			return
		}
	}

	//TODO: handle closed channel

	if lthread.paused.CompareAndSwap(false, true) {
		unlock = false
		lthread.lock.Unlock()

		if ctx != lthread.state.Ctx {
			panic(ErrUnreachable)
		}

		ctx.DoIO(func() error {
			<-lthread.continueExecChan
			return nil
		})

		lthread.paused.Store(false)
	} else {
		panic(errors.New(".paused should not be true"))
	}
}

func (lthread *LThread) IsPaused() bool {
	return lthread.paused.Load()
}

func (lthread *LThread) ResumeAsync() error {
	if lthread.IsDone() {
		return ErrLThreadIsDone
	}

	lthread.lock.Lock()
	defer lthread.lock.Unlock()

	if !lthread.paused.Load() {
		return nil
	}

	select {
	case lthread.continueExecChan <- struct{}{}:
	default:
	}

	return nil
}

// Cancel stops the execution of the lthread.
func (lthread *LThread) Cancel(*Context) {
	lthread.state.Ctx.CancelGracefully()
}

// WaitResult waits for the end of the execution and returns the value returned by
func (lthread *LThread) WaitResult(ctx *Context) (Value, error) {
	if lthread.IsDone() {
		lthread.lock.Lock()
		defer lthread.lock.Unlock()

		if lthread.err.goError != nil {
			return nil, lthread.err.goError
		}
		return lthread.result, nil
	}
	ctx.DoIO(func() error {
		<-lthread.wait_result
		return nil
	})
	close(lthread.wait_result)

	if lthread.err.goError != nil {
		return nil, lthread.err.goError
	}
	return lthread.result, nil
}

// A LThreadGroup is a group of lthreads, it simplifies the interaction with the lthreads.
type LThreadGroup struct {
	threads []*LThread
}

func NewLThreadGroup(ctx *Context) *LThreadGroup {
	return &LThreadGroup{}
}

func (g *LThreadGroup) GetGoMethod(name string) (*GoFunction, bool) {
	switch name {
	case "wait_results":
		return WrapGoMethod(g.WaitAllResults), true
	case "cancel_all":
		return WrapGoMethod(g.CancelAll), true
	}
	return nil, false
}

func (g *LThreadGroup) Prop(ctx *Context, name string) Value {
	method, ok := g.GetGoMethod(name)
	if !ok {
		panic(FormatErrPropertyDoesNotExist(name, g))
	}
	return method
}

func (*LThreadGroup) SetProp(ctx *Context, name string, value Value) error {
	return ErrCannotSetProp
}

func (*LThreadGroup) PropertyNames(ctx *Context) []string {
	return ROUTINE_GROUP_PROPNAMES
}

func (group *LThreadGroup) Add(newRt *LThread) {
	for _, rt := range group.threads {
		if rt == newRt {
			panic(errors.New("attempt to add a lthread to a group more than once"))
		}
	}
	group.threads = append(group.threads, newRt)
}

// WaitAllResults waits for the results of all threads in the group and returns a list of results.
func (group *LThreadGroup) WaitAllResults(ctx *Context) (*Array, error) {
	results := Array{}

	for _, rt := range group.threads {
		rtRes, rtErr := rt.WaitResult(ctx)
		if rtErr != nil {
			return nil, rtErr
		}
		results = append(results, rtRes)
	}

	return &results, nil
}

// CancelAll stops the execution of all threads in the group.
func (group *LThreadGroup) CancelAll(*Context) {
	for _, lthread := range group.threads {
		lthread.Cancel(nil)
	}
}

type ExecutedStep struct {
	result  Value
	endTime time.Time
}

func (s *ExecutedStep) GetGoMethod(name string) (*GoFunction, bool) {
	return nil, false
}

func (s *ExecutedStep) Prop(ctx *Context, name string) Value {
	switch name {
	case "result":
		return s.result
	case "end_time":
		return Date(s.endTime)
	}
	method, ok := s.GetGoMethod(name)
	if !ok {
		panic(FormatErrPropertyDoesNotExist(name, s))
	}
	return method
}

func (*ExecutedStep) SetProp(ctx *Context, name string, value Value) error {
	return ErrCannotSetProp
}

func (*ExecutedStep) PropertyNames(ctx *Context) []string {
	return EXECUTED_STEP_PROPNAMES
}

func Sleep(ctx *Context, d Duration) {
	ctx.Sleep(time.Duration(d))
}

func readLThreadMeta(meta map[string]Value, ctx *Context) (group *LThreadGroup, globalsDesc Value, permListing *Object, err error) {
	if val, ok := meta[symbolic.LTHREAD_META_GROUP_SECTION]; ok {
		if rtGroup, ok := val.(*LThreadGroup); ok {
			group = rtGroup
		} else {
			return nil, nil, nil, fmt.Errorf("<meta>.%s should be a lthread group", symbolic.LTHREAD_META_GROUP_SECTION)
		}
	}
	if val, ok := meta[symbolic.LTHREAD_META_GLOBALS_SECTION]; ok {
		globalsDesc = val
	}
	if val, ok := meta[symbolic.LTHREAD_META_ALLOW_SECTION]; ok {
		if obj, ok := val.(*Object); ok {
			permListing = obj
		} else {
			return nil, nil, nil, fmt.Errorf("<meta>.%s should be an object", symbolic.LTHREAD_META_ALLOW_SECTION)
		}
	}

	return
}
