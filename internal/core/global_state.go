package internal

import (
	"errors"
	"io"
	"log"

	symbolic "github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/utils"
)

var (
	ErrContextInUse = errors.New("cannot create a new global state with a context that already has an associated state")

	GLOBAL_STATE_PROPNAMES = []string{"module"}
)

// A GlobalState represents the global state for the evaluation of a single module or the shell's loop.
type GlobalState struct {
	Ctx         *Context
	Module      *Module         //nil in some cases (shell, mapping entry's state), TODO: check for usage
	Globals     GlobalVariables //global variables
	Routine     *Routine        //not nil if running in a routine
	SystemGraph *SystemGraph
	Out         io.Writer   //nil by default
	Logger      *log.Logger //nil by default

	StaticCheckData *StaticCheckData
	SymbolicData    *SymbolicData
	LockedValues    []PotentiallySharable

	NotClonableMixin
	NoReprMixin
}

func NewGlobalState(ctx *Context, args ...map[string]Value) *GlobalState {
	if ctx.state != nil {
		panic(ErrContextInUse)
	}

	state := &GlobalState{
		Globals:      GlobalVariablesFromMap(map[string]Value{}),
		Ctx:          ctx,
		SymbolicData: &SymbolicData{SymbolicData: symbolic.NewSymbolicData()},
	}
	ctx.state = state

	for _, arg := range args {
		for k, v := range arg {
			state.Globals.permanent[k] = v
		}
	}

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
	permanent            map[string]Value
	capturedGlobalsStack [][]capturedGlobal
}

func GlobalVariablesFromMap(m map[string]Value) GlobalVariables {
	return GlobalVariables{permanent: m}
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

func (g *GlobalVariables) Foreach(fn func(name string, v Value)) {
	if len(g.capturedGlobalsStack) != 0 {
		for _, captured := range g.capturedGlobalsStack[len(g.capturedGlobalsStack)-1] {
			fn(captured.name, captured.value)
		}
	}

	for k, v := range g.permanent {
		fn(k, v)
	}
}

func (g *GlobalVariables) Set(name string, value Value) {
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
