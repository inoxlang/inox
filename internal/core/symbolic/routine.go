package symbolic

import (
	"bufio"

	pprint "github.com/inoxlang/inox/internal/pretty_print"
	"github.com/inoxlang/inox/internal/utils"
)

var (
	ROUTINE_PROPNAMES       = []string{"wait_result", "cancel", "steps"}
	ROUTINE_GROUP_PROPNAMES = []string{"wait_results", "cancel_all"}
	EXECUTED_STEP_PROPNAMES = []string{"result", "end_time"}

	ANY_ROUTINE       = &Routine{}
	ANY_ROUTINE_GROUP = &RoutineGroup{}
	ANY_EXECUTED_STEP = &ExecutedStep{}
)

// A Routine represents a symbolic Routine.
type Routine struct {
	UnassignablePropsMixin
	_ int
}

func (r *Routine) Test(v SymbolicValue) bool {
	switch v.(type) {
	case *Routine:
		return true
	default:
		return false
	}
}

func (r *Routine) WidestOfType() SymbolicValue {
	return ANY_ROUTINE
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

func (r *Routine) Prop(name string) SymbolicValue {
	switch name {
	case "steps":
		return NewArrayOf(&ExecutedStep{})
	}
	method, ok := r.GetGoMethod(name)
	if !ok {
		panic(FormatErrPropertyDoesNotExist(name, r))
	}
	return method
}

func (*Routine) PropertyNames() []string {
	return ROUTINE_PROPNAMES
}

func (routine *Routine) WaitResult(ctx *Context) (SymbolicValue, *Error) {
	return ANY, nil
}

func (routine *Routine) Cancel(*Context) {

}

func (r *Routine) Widen() (SymbolicValue, bool) {
	return nil, false
}

func (r *Routine) IsWidenable() bool {
	return false
}

func (r *Routine) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%routine")))
}

// A RoutineGroup represents a symbolic RoutineGroup.
type RoutineGroup struct {
	UnassignablePropsMixin
	_ int
}

func (g *RoutineGroup) Test(v SymbolicValue) bool {
	switch v.(type) {
	case *RoutineGroup:
		return true
	default:
		return false
	}
}

func (g *RoutineGroup) GetGoMethod(name string) (*GoFunction, bool) {
	switch name {
	case "wait_results":
		return WrapGoMethod(g.WaitAllResults), true
	case "cancel_all":
		return WrapGoMethod(g.CancelAll), true
	}
	return nil, false
}

func (g *RoutineGroup) Prop(name string) SymbolicValue {
	method, ok := g.GetGoMethod(name)
	if !ok {
		panic(FormatErrPropertyDoesNotExist(name, g))
	}
	return method
}

func (*RoutineGroup) PropertyNames() []string {
	return ROUTINE_GROUP_PROPNAMES
}

func (g *RoutineGroup) Add(newRt *Routine) {

}

func (g *RoutineGroup) WaitAllResults(ctx *Context) (*Array, *Error) {
	return NewAnyArray(), nil
}

func (g *RoutineGroup) CancelAll(*Context) {

}

func (g *RoutineGroup) Widen() (SymbolicValue, bool) {
	return nil, false
}

func (g *RoutineGroup) IsWidenable() bool {
	return false
}

func (g *RoutineGroup) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%routine-group")))
}

func (g *RoutineGroup) WidestOfType() SymbolicValue {
	return ANY_ROUTINE_GROUP
}

// An ExecutedStep represents a symbolic ExecutedStep.
type ExecutedStep struct {
	UnassignablePropsMixin
	_ int
}

func (s *ExecutedStep) Test(v SymbolicValue) bool {
	switch v.(type) {
	case *ExecutedStep:
		return true
	default:
		return false
	}
}

func (s *ExecutedStep) WidestOfType() SymbolicValue {
	return ANY_EXECUTED_STEP
}

func (s *ExecutedStep) GetGoMethod(name string) (*GoFunction, bool) {
	return nil, false
}

func (s *ExecutedStep) Prop(name string) SymbolicValue {
	switch name {
	case "result":
		return ANY
	case "end_time":
		return ANY_DATE
	}

	method, ok := s.GetGoMethod(name)
	if !ok {
		panic(FormatErrPropertyDoesNotExist(name, s))
	}
	return method
}

func (*ExecutedStep) PropertyNames() []string {
	return EXECUTED_STEP_PROPNAMES
}

func (s *ExecutedStep) WaitResult(ctx *Context) (SymbolicValue, *Error) {
	return ANY, nil
}

func (s *ExecutedStep) Cancel(*Context) {
}

func (s *ExecutedStep) Widen() (SymbolicValue, bool) {
	return nil, false
}

func (s *ExecutedStep) IsWidenable() bool {
	return false
}

func (s *ExecutedStep) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%executed-step")))
}
