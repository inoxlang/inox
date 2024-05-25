package core

import (
	"errors"
	"fmt"
	"io"
	"reflect"
	"slices"
	"sync"
	"sync/atomic"

	"github.com/inoxlang/inox/internal/core/mem"
	"github.com/inoxlang/inox/internal/core/slog"
	"github.com/inoxlang/inox/internal/core/staticcheck"
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/rs/zerolog"
	"golang.org/x/exp/maps"
)

const (
	MINIMAL_STATE_ID             = 1
	INITIAL_MODULE_HEAP_CAPACITY = 1000

	MINIMUM_MOD_PRIORITY   = ModulePriority(10)
	READ_TX_PRIORITY       = ModulePriority(50)
	READ_WRITE_TX_PRIORITY = ModulePriority(150)
)

var (
	ErrContextInUse           = errors.New("cannot create a new global state with a context that already has an associated state")
	ErrOutAndLoggerAlreadySet = errors.New(".Out & .Logger are already definitely set")

	GLOBAL_STATE_PROPNAMES = []string{"module"}
	previousStateId        atomic.Int64
)

// A GlobalState represents the global state of a module (or the shell loop), most exported fields should be set once.
// Patterns and host definition data are stored in the context.
type GlobalState struct {
	id     StateId
	Module *Module //nil in some cases (e.g. shell, mapping entry's state), TODO: check for usage

	//Output and logs

	Out                     io.Writer      //io.Discard by default
	Logger                  zerolog.Logger //zerolog.Nop() by default
	LogLevels               *slog.Levels   //DEFAULT_LOG_LEVELS by default
	OutputFieldsInitialized atomic.Bool    //should be set to true by the state's creator, even if the default values are kept.

	//Most relevant components

	Ctx             *Context
	Manifest        *Manifest
	MemberAuthToken string //Can be empty. Most of the time this field is only set for the main state.. //TODO: replace with a JWT.
	//Bytecode        *Bytecode              //can be nil
	Globals      GlobalVariables //global variables
	LThread      *LThread        //not nil if running in a dedicated LThread
	Heap         *mem.ModuleHeap
	lockedValues []PotentiallySharable

	//Re-usable buffers for Go function calls made by reflect.Call.

	goCallArgPrepBuf []any
	goCallArgsBuf    []reflect.Value

	//Related states

	MainState            *GlobalState //never nil except for parents of main states. This field should be set by the user of GlobalState.
	descendantStates     map[ResourceName]*GlobalState
	descendantStatesLock sync.Mutex

	//Factories for building the state of any imported module

	GetBaseGlobalsForImportedModule      func(ctx *Context, manifest *Manifest) (GlobalVariables, error) // ok if nil
	GetBasePatternsForImportedModule     func() (map[string]Pattern, map[string]*PatternNamespace)       // return nil maps by default
	SymbolicBaseGlobalsForImportedModule map[string]symbolic.Value                                       // ok if nil, should not be modified

	//Debugging and testing

	Debugger atomic.Value //nil or (nillable) *Debugger
	//TODO: TestingState TestingState

	//Errors & check data

	PrenitStaticCheckErrors []*staticcheck.Error
	MainPreinitError        error
	StaticCheckData         *StaticCheckData
	SymbolicData            *SymbolicData
	FinalSymbolicCheckError error

	//Information on preparation

	EffectivePreparationParameters EffectivePreparationParameters
}

type StateId int64

type ModulePriority uint32

// NewGlobalState creates a state with the provided context and constants.
// The OutputFieldsInitialized field is not initialized and should be set by the caller.
func NewGlobalState(ctx *Context, constants ...map[string]Value) *GlobalState {
	if ctx.state != nil {
		panic(ErrContextInUse)
	}

	state := &GlobalState{
		id:               StateId(previousStateId.Add(1)),
		Ctx:              ctx,
		SymbolicData:     &SymbolicData{Data: symbolic.NewSymbolicData()},
		descendantStates: make(map[ResourceName]*GlobalState, 0),

		Out:       io.Discard,
		Logger:    zerolog.Nop(),
		LogLevels: slog.DEFAULT_LEVELS,

		GetBasePatternsForImportedModule: func() (map[string]Pattern, map[string]*PatternNamespace) {
			return nil, nil
		},

		goCallArgPrepBuf: make([]any, 10),
		goCallArgsBuf:    make([]reflect.Value, 10),

		//TODO: use a heap type suited to the module's type and its expected lifespan.
		Heap: mem.NewArenaHeap(INITIAL_MODULE_HEAP_CAPACITY),
	}
	ctx.SetClosestState(state)

	globals := map[string]Value{}

	for _, arg := range constants {
		for k, v := range arg {
			if v.IsMutable() {
				panic(fmt.Errorf("error while creating a new state: constant global %s is mutable", k))
			}
			globals[k] = v
		}
	}

	state.Globals = GlobalVariablesFromMap(globals, maps.Keys(globals))

	return state
}

// IsMain returns true if g.MainState == g.
func (g *GlobalState) IsMain() bool {
	if g.MainState == nil {
		panic(ErrUnreachable)
	}

	return g.MainState == g
}

func (g *GlobalState) SetDescendantState(src ResourceName, state *GlobalState) {
	g.descendantStatesLock.Lock()
	defer g.descendantStatesLock.Unlock()

	if _, ok := g.descendantStates[src]; ok {
		panic(fmt.Errorf("descendant state of %s already set", src.ResourceName()))
	}
	g.descendantStates[src] = state
	if g.MainState != nil && g.MainState != g {
		g.MainState.SetDescendantState(src, state)
	}
}

func (g *GlobalState) ComputePriority() ModulePriority {
	tx := g.Ctx.GetTx()
	if tx == nil {
		return MINIMUM_MOD_PRIORITY
	}
	if tx.IsReadonly() {
		return READ_TX_PRIORITY
	}
	return READ_WRITE_TX_PRIORITY
}

func (g *GlobalState) GetGoMethod(name string) (*GoFunction, bool) {
	return nil, false
}

func (g *GlobalState) Prop(ctx *Context, name string) Value {
	switch name {
	case "module":
		return g.Module
	}
	method, ok := g.GetGoMethod(name)
	if !ok {
		panic(FormatErrPropertyDoesNotExist(name, g))
	}
	return method
}

func (*GlobalState) SetProp(ctx *Context, name string, value Value) error {
	return ErrCannotSetProp
}

func (*GlobalState) PropertyNames(ctx *Context) []string {
	return ROUTINE_GROUP_PROPNAMES
}

// A GlobalVariables represents the global scope of a module.
// Global variables captured by shared Inox functions are temporarily added during calls.
type GlobalVariables struct {
	//start constants are all the constants set before the code starts its execution,
	//therefore that include base constants & constants defined before the manifest.
	startConstants []string

	permanent            map[string]Value
	capturedGlobalsStack [][]capturedGlobal
}

func GlobalVariablesFromMap(m map[string]Value, startConstants []string) GlobalVariables {
	if m == nil {
		m = make(map[string]Value)
	}
	return GlobalVariables{permanent: m, startConstants: startConstants}
}

func (g *GlobalVariables) Get(name string) Value {
	if len(g.capturedGlobalsStack) != 0 {
		for _, captured := range g.capturedGlobalsStack[len(g.capturedGlobalsStack)-1] {
			if captured.name == name {
				return captured.value
			}
		}
	}

	return g.permanent[name]
}

func (g *GlobalVariables) CheckedGet(name string) (Value, bool) {
	if len(g.capturedGlobalsStack) != 0 {
		for _, captured := range g.capturedGlobalsStack[len(g.capturedGlobalsStack)-1] {
			if captured.name == name {
				return captured.value, true
			}
		}
	}
	v, ok := g.permanent[name]
	return v, ok
}

func (g *GlobalVariables) Has(name string) bool {
	if len(g.capturedGlobalsStack) != 0 {
		for _, captured := range g.capturedGlobalsStack[len(g.capturedGlobalsStack)-1] {
			if captured.name == name {
				return true
			}
		}
	}
	_, ok := g.permanent[name]
	return ok
}

func (g *GlobalVariables) Foreach(fn func(name string, v Value, isStartConstant bool) error) error {

	if len(g.capturedGlobalsStack) != 0 {
		for _, captured := range g.capturedGlobalsStack[len(g.capturedGlobalsStack)-1] {
			err := fn(captured.name, captured.value, false) //TODO: never a constant global ?
			if err != nil {
				return err
			}
		}
	}

	for k, v := range g.permanent {
		isStartConstant := slices.Contains(g.startConstants, k)
		err := fn(k, v, isStartConstant)
		if err != nil {
			return err
		}
	}
	return nil
}

// Set sets the value for a global variable (not constant)
func (g *GlobalVariables) Set(name string, value Value) {

	if slices.Contains(g.startConstants, name) {
		panic(fmt.Errorf("cannot change value of global constant %s", name))
	}

	if len(g.capturedGlobalsStack) != 0 {
		for _, captured := range g.capturedGlobalsStack[len(g.capturedGlobalsStack)-1] {
			if captured.name == name {
				panic(ErrAttemptToSetCaptureGlobal)
			}
		}
	}

	g.permanent[name] = value
}

// SetChecked sets the value for a global variable (not constant), it called
func (g *GlobalVariables) SetCheck(name string, value Value, allow func(defined bool) error) error {
	if slices.Contains(g.startConstants, name) {
		panic(fmt.Errorf("cannot change value of global constant %s", name))
	}

	if len(g.capturedGlobalsStack) != 0 {
		for _, captured := range g.capturedGlobalsStack[len(g.capturedGlobalsStack)-1] {
			if captured.name == name {
				panic(ErrAttemptToSetCaptureGlobal)
			}
		}
	}

	_, alreadyDefined := g.permanent[name]
	if err := allow(alreadyDefined); err != nil {
		return err
	}
	g.permanent[name] = value
	return nil
}

func (g *GlobalVariables) PushCapturedGlobals(captured []capturedGlobal) {
	g.capturedGlobalsStack = append(g.capturedGlobalsStack, captured)
}

func (g *GlobalVariables) PopCapturedGlobals() {
	g.capturedGlobalsStack = g.capturedGlobalsStack[:len(g.capturedGlobalsStack)-1]
}

func (g *GlobalVariables) Entries() map[string]Value {
	_map := maps.Clone(g.permanent)

	if len(g.capturedGlobalsStack) != 0 {
		for _, captured := range g.capturedGlobalsStack[len(g.capturedGlobalsStack)-1] {
			_map[captured.name] = captured.value
		}
	}

	return _map
}

func (g *GlobalVariables) Constants() map[string]Value {
	constants := make(map[string]Value, len(g.startConstants))
	for _, name := range g.startConstants {
		val := g.permanent[name]
		if val == nil {
			panic(fmt.Errorf("value of constant %s is nil", val))
		}
		constants[name] = val
	}

	return constants
}
