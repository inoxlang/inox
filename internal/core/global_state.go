package core

import (
	"errors"
	"fmt"
	"io"

	symbolic "github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/rs/zerolog"
)

var (
	ErrContextInUse = errors.New("cannot create a new global state with a context that already has an associated state")

	GLOBAL_STATE_PROPNAMES = []string{"module"}
)

// A GlobalState represents the global state for the evaluation of a single module or the shell's loop.
type GlobalState struct {
	Ctx          *Context
	Module       *Module                //nil in some cases (shell, mapping entry's state), TODO: check for usage
	Globals      GlobalVariables        //global variables
	Routine      *Routine               //not nil if running in a routine
	Databases    map[string]*DatabaseIL //the map should never change
	SystemGraph  *SystemGraph
	lockedValues []PotentiallySharable

	GetBaseGlobalsForImportedModule  func(ctx *Context, manifest *Manifest) (GlobalVariables, error) // ok if nil
	GetBasePatternsForImportedModule func() (map[string]Pattern, map[string]*PatternNamespace)       // ok if nil
	Out                              io.Writer                                                       //nil by default
	Logger                           zerolog.Logger                                                  //nil by default

	//errors & check data
	PrenitStaticCheckErrors []*StaticCheckError
	MainPreinitError        error
	StaticCheckData         *StaticCheckData
	SymbolicData            *SymbolicData

	NotClonableMixin
	NoReprMixin
}

func NewGlobalState(ctx *Context, constants ...map[string]Value) *GlobalState {
	if ctx.state != nil {
		panic(ErrContextInUse)
	}

	state := &GlobalState{
		Ctx:          ctx,
		SymbolicData: &SymbolicData{SymbolicData: symbolic.NewSymbolicData()},
	}
	ctx.state = state

	globals := map[string]Value{}

	for _, arg := range constants {
		for k, v := range arg {
			if v.IsMutable() {
				panic(fmt.Errorf("error while creating a new state: constant global %s is mutable", k))
			}
			globals[k] = v
		}
	}

	state.Globals = GlobalVariablesFromMap(globals, utils.GetMapKeys(globals))

	return state
}

func (g *GlobalState) InitSystemGraph() {
	if g.SystemGraph != nil {
		return
	}
	g.SystemGraph = NewSystemGraph()
}

func (g *GlobalState) ProposeSystemGraph(v SystemGraphNodeValue, optionalName string) {
	if g.SystemGraph != nil {
		v.ProposeSystemGraph(g.Ctx, g.SystemGraph, optionalName, nil)
	}
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
		isStartConstant := utils.SliceContains(g.startConstants, k)
		err := fn(k, v, isStartConstant)
		if err != nil {
			return err
		}
	}
	return nil
}

// Set set the value for a global variable (not constant)
func (g *GlobalVariables) Set(name string, value Value) {

	if utils.SliceContains(g.startConstants, name) {
		panic(fmt.Errorf("cannot change value of global constant %s", name))
	}

	if len(g.capturedGlobalsStack) != 0 {
		for _, captured := range g.capturedGlobalsStack[len(g.capturedGlobalsStack)-1] {
			if captured.name == name {
				panic(errors.New("attempt to set a captured global"))
			}
		}
	}

	g.permanent[name] = value
}

func (g *GlobalVariables) PushCapturedGlobals(captured []capturedGlobal) {
	g.capturedGlobalsStack = append(g.capturedGlobalsStack, captured)
}

func (g *GlobalVariables) PopCapturedGlobals() {
	g.capturedGlobalsStack = g.capturedGlobalsStack[:len(g.capturedGlobalsStack)-1]
}

func (g *GlobalVariables) Entries() map[string]Value {
	_map := utils.CopyMap(g.permanent)

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
