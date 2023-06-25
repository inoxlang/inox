package core

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/inoxlang/inox/internal/core/symbolic"
	parse "github.com/inoxlang/inox/internal/parse"
	permkind "github.com/inoxlang/inox/internal/permkind"
	"github.com/inoxlang/inox/internal/utils"
)

const ROUTINE_POST_YIELD_PAUSE = time.Microsecond

var (
	ROUTINE_PROPNAMES       = []string{"wait_result", "cancel", "steps"}
	ROUTINE_GROUP_PROPNAMES = []string{"wait_results", "cancel_all"}
	EXECUTED_STEP_PROPNAMES = []string{"result", "end_time"}
)

func init() {
	RegisterSymbolicGoFunction(NewRoutineGroup, func(xtx *symbolic.Context) *symbolic.RoutineGroup {
		return &symbolic.RoutineGroup{}
	})
}

// A Routine is similar to a goroutine in Golang, it represents of the execution of a single module and can be cancelled at any time.
type Routine struct {
	NoReprMixin
	NotClonableMixin

	useBytecode bool
	module      *Module
	state       *GlobalState
	bytecode    *Bytecode //nil if tree walking evaluation
	lock        sync.Mutex

	//steps
	executedSteps          []*ExecutedStep
	executedStepCallbackFn func(step ExecutedStep, routine *Routine) (continueExec bool)
	continueExecChan       chan struct{}
	paused                 atomic.Bool

	//result
	result      Value
	err         Error
	done        atomic.Bool
	wait_result chan struct{}
}

type RoutineSpawnArgs struct {
	SpawnerState *GlobalState
	Globals      GlobalVariables
	Module       *Module
	RoutineCtx   *Context
	PreinitState *GlobalState

	//AbsScriptDir string
	UseBytecode bool
	StartPaused bool
	Self        Value
	Timeout     time.Duration

	IgnoreCreateRoutinePermCheck bool
}

// SpawnRoutines spawns a new routine, if .routineCtx is nil a minimal context is created for the routine.
// The provided globals that are not thread safe are wrapped in a SharedValue.
func SpawnRoutine(args RoutineSpawnArgs) (*Routine, error) {

	if !args.IgnoreCreateRoutinePermCheck {
		perm := RoutinePermission{Kind_: permkind.Create}

		if err := args.SpawnerState.Ctx.CheckHasPermission(perm); err != nil {
			return nil, fmt.Errorf("cannot spawn routine: %s", err.Error())
		}
	}

	if args.RoutineCtx == nil {
		args.RoutineCtx = NewContext(ContextConfig{
			Permissions: []Permission{
				GlobalVarPermission{Kind_: permkind.Read, Name: "*"},
				GlobalVarPermission{Kind_: permkind.Use, Name: "*"},
				GlobalVarPermission{Kind_: permkind.Create, Name: "*"},
			},
			ParentContext: args.SpawnerState.Ctx,
		})
	}

	staticCheckData, err := StaticCheck(StaticCheckInput{
		Node:              args.Module.MainChunk.Node,
		Module:            args.Module,
		Chunk:             args.Module.MainChunk,
		Globals:           args.Globals,
		Patterns:          args.RoutineCtx.namedPatterns,
		PatternNamespaces: args.RoutineCtx.patternNamespaces,
	})
	if err != nil {
		return nil, fmt.Errorf("cannot spawn routine: expression: module/expr checking failed: %w", err)
	}

	//set global variables & constants
	modState := NewGlobalState(args.RoutineCtx, args.Globals.Constants())
	err = args.Globals.Foreach(func(name string, v Value, isConstant bool) error {
		if isConstant {
			return nil
		}
		shared, err := ShareOrClone(v, args.SpawnerState)
		if err != nil {
			return fmt.Errorf("failed to share/clone provided global '%s': %w", name, err)
		}
		modState.Globals.Set(name, shared)
		return nil
	})
	if err != nil {
		return nil, err
	}

	modState.Module = args.Module
	modState.Logger = args.SpawnerState.Logger
	modState.Out = args.SpawnerState.Out
	modState.StaticCheckData = staticCheckData
	modState.GetBaseGlobalsForImportedModule = args.SpawnerState.GetBaseGlobalsForImportedModule
	modState.GetBasePatternsForImportedModule = args.SpawnerState.GetBasePatternsForImportedModule
	// TODO: set SymbolicData

	routine := &Routine{
		module:           args.Module,
		state:            modState,
		wait_result:      make(chan struct{}, 1),
		continueExecChan: make(chan struct{}, 1),
		bytecode:         args.Module.Bytecode,
		useBytecode:      args.UseBytecode,
		executedStepCallbackFn: func(step ExecutedStep, routine *Routine) (continueExec bool) {
			return true
		},
	}

	modState.Routine = routine

	if args.Timeout != 0 {
		go func(d time.Duration) {
			<-time.After(d)
			modState.Ctx.Cancel()
		}(args.Timeout)
	}

	if args.StartPaused {
		routine.paused.Store(true)
	}

	// goroutine in which the routine's module is evaluated
	go func(modState *GlobalState, chunk parse.Node, routine *Routine, startPaused bool, self Value) {
		var res Value
		var err error

		defer func() {
			e := recover()
			if v, ok := e.(error); ok {
				err = v
			}

			routine.lock.Lock()
			defer routine.lock.Unlock()

			close(routine.continueExecChan)
			routine.done.Store(true)
			routine.paused.Store(false)

			if res == nil {
				res = Nil
			}

			if err == nil {
				res, err = ShareOrClone(res, routine.state)
			}

			if err != nil {
				modState.Logger.Print("a routine failed or was cancelled: " + utils.AddCarriageReturnAfterNewlines(err.Error()))
				routine.result = Nil
				routine.err = ValOf(err).(Error)
				routine.wait_result <- struct{}{}
				return
			}

			routine.result = res
			routine.wait_result <- struct{}{}
		}()

		if startPaused {
			select {
			case <-routine.continueExecChan:
				//
			case <-modState.Ctx.Done():
				panic(context.Canceled)
			}
		}

		if args.UseBytecode {
			res, err = EvalBytecode(routine.bytecode, modState, args.Self)
		} else {
			state := NewTreeWalkStateWithGlobal(modState)
			state.self = args.Self
			res, err = TreeWalkEval(chunk, state)
		}

	}(modState, args.Module.MainChunk.Node, routine, args.StartPaused, args.Self)

	return routine, nil
}

func (r *Routine) GetGoMethod(name string) (*GoFunction, bool) {
	switch name {
	case "wait_result":
		return WrapGoMethod(r.WaitResult), true
	case "cancel":
		return WrapGoMethod(r.Cancel), true
	}
	return nil, false
}

func (r *Routine) Prop(ctx *Context, name string) Value {
	switch name {
	case "steps":
		r.lock.Lock()
		defer r.lock.Unlock()
		steps := make([]Value, len(r.executedSteps))

		for i := 0; i < len(r.executedSteps); i++ {
			steps[i] = r.executedSteps[i]
		}
		return NewWrappedValueList(steps...)
	}
	method, ok := r.GetGoMethod(name)
	if !ok {
		panic(FormatErrPropertyDoesNotExist(name, r))
	}
	return method
}

func (*Routine) SetProp(ctx *Context, name string, value Value) error {
	return ErrCannotSetProp
}

func (*Routine) PropertyNames(ctx *Context) []string {
	return ROUTINE_PROPNAMES
}

// Cancel stops the execution of the routine.
func (routine *Routine) IsDone() bool {
	return routine.done.Load()
}

// yield creates a new ExecutedStep with the given result, if a the step callback function is set it is executed
// to determinate if execution will be paused, if not set the execution is paused.
func (routine *Routine) yield(ctx *Context, value Value) {
	if routine.IsDone() {
		panic(ErrRoutineIsDone)
	}

	routine.lock.Lock()
	defer routine.lock.Unlock()

	step := &ExecutedStep{
		result:  value,
		endTime: time.Now(),
	}

	routine.executedSteps = append(routine.executedSteps, step)

	if routine.executedStepCallbackFn != nil {
		if continueExec := routine.executedStepCallbackFn(*step, routine); continueExec {
			time.Sleep(ROUTINE_POST_YIELD_PAUSE)
			return
		}
	}

	//TODO: handle closed channel

	if routine.paused.CompareAndSwap(false, true) {
		routine.lock.Unlock()
		<-routine.continueExecChan
		routine.paused.Store(false)
	} else {
		panic(errors.New(".paused should not be true"))
	}
}

func (routine *Routine) IsPaused() bool {
	return routine.paused.Load()
}

func (routine *Routine) ResumeAsync() error {
	if routine.IsDone() {
		return ErrRoutineIsDone
	}

	routine.lock.Lock()
	defer routine.lock.Unlock()

	if !routine.paused.Load() {
		return nil
	}

	if len(routine.continueExecChan) == 0 {
		routine.continueExecChan <- struct{}{}
	}
	return nil
}

// Cancel stops the execution of the routine.
func (routine *Routine) Cancel(*Context) {
	routine.state.Ctx.Cancel()
}

// WaitResult waits for the end of the execution and returns the value returned by
func (routine *Routine) WaitResult(ctx *Context) (Value, error) {
	if routine.IsDone() {
		routine.lock.Lock()
		defer routine.lock.Unlock()

		if routine.err.goError != nil {
			return nil, routine.err.goError
		}
		return routine.result, nil
	}
	<-routine.wait_result
	close(routine.wait_result)

	if routine.err.goError != nil {
		return nil, routine.err.goError
	}
	return routine.result, nil
}

// A RoutineGroup is a group of routines, it simplifies the interaction with the routines.
type RoutineGroup struct {
	NoReprMixin
	NotClonableMixin

	routines []*Routine
}

func NewRoutineGroup(ctx *Context) *RoutineGroup {
	return &RoutineGroup{}
}

func (g *RoutineGroup) GetGoMethod(name string) (*GoFunction, bool) {
	switch name {
	case "wait_results":
		return &GoFunction{fn: g.WaitAllResults}, true
	case "cancel_all":
		return &GoFunction{fn: g.CancelAll}, true
	}
	return nil, false
}

func (g *RoutineGroup) Prop(ctx *Context, name string) Value {
	method, ok := g.GetGoMethod(name)
	if !ok {
		panic(FormatErrPropertyDoesNotExist(name, g))
	}
	return method
}

func (*RoutineGroup) SetProp(ctx *Context, name string, value Value) error {
	return ErrCannotSetProp
}

func (*RoutineGroup) PropertyNames(ctx *Context) []string {
	return ROUTINE_GROUP_PROPNAMES
}

func (group *RoutineGroup) Add(newRt *Routine) {
	for _, rt := range group.routines {
		if rt == newRt {
			panic(errors.New("attempt to add a routine to a group more than once"))
		}
	}
	group.routines = append(group.routines, newRt)
}

// WaitAllResults waits for the results of all routines in the group and returns a list of results.
func (group *RoutineGroup) WaitAllResults(ctx *Context) (*List, error) {
	results := &ValueList{}

	for _, rt := range group.routines {
		rtRes, rtErr := rt.WaitResult(ctx)
		if rtErr != nil {
			return nil, rtErr
		}
		results.elements = append(results.elements, rtRes)
	}

	return WrapUnderylingList(results), nil
}

// CancelAll stops the execution of all routines in the group.
func (group *RoutineGroup) CancelAll(*Context) {
	for _, routine := range group.routines {
		routine.Cancel(nil)
	}
}

type ExecutedStep struct {
	NoReprMixin
	NotClonableMixin

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
	select {
	case <-ctx.Done():
		return
	case <-time.After(time.Duration(d)):
		// add pause ?
	}
}

func readRoutineMeta(meta Value, ctx *Context) (group *RoutineGroup, globalsDesc Value, permListing *Object, err error) {
	if obj, ok := meta.(*Object); ok {
		if obj.HasProp(ctx, "group") {
			val := obj.Prop(ctx, "group")
			if rtGroup, ok := val.(*RoutineGroup); ok {
				group = rtGroup
			} else {
				return nil, nil, nil, errors.New("<meta>.group should be a routine group")
			}
		}
		if obj.HasProp(ctx, "globals") {
			globalsDesc = obj.Prop(ctx, "globals")
		}
		if obj.HasProp(ctx, "allow") {
			val := obj.Prop(ctx, "allow")
			if obj, ok := val.(*Object); ok {
				permListing = obj
			} else {
				return nil, nil, nil, errors.New("<meta>.allow should be an object")
			}
		}
	} else {
		return nil, nil, nil, errors.New("<meta should be an object")
	}

	return
}
