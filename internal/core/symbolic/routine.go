package internal

var (
	ROUTINE_PROPNAMES       = []string{"wait_result", "cancel", "steps"}
	ROUTINE_GROUP_PROPNAMES = []string{"wait_results", "cancelAll"}
	EXECUTED_STEP_PROPNAMES = []string{"result", "end_time"}
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
	return &Routine{}
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
		return NewListOf(&ExecutedStep{})
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

func (r *Routine) String() string {
	return "routine"
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
	case "cancelAll":
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

func (g *RoutineGroup) WaitAllResults(ctx *Context) (*List, *Error) {
	return NewList(), nil
}

func (g *RoutineGroup) CancelAll(*Context) {

}

func (g *RoutineGroup) Widen() (SymbolicValue, bool) {
	return nil, false
}

func (g *RoutineGroup) IsWidenable() bool {
	return false
}

func (g *RoutineGroup) String() string {
	return "routine-group"
}

func (g *RoutineGroup) WidestOfType() SymbolicValue {
	return &RoutineGroup{}
}

// An ExecutedStep represents a symbolic ExecutedStep.
type ExecutedStep struct {
	UnassignablePropsMixin
	_ int
}

func (r *ExecutedStep) Test(v SymbolicValue) bool {
	switch v.(type) {
	case *ExecutedStep:
		return true
	default:
		return false
	}
}

func (r *ExecutedStep) WidestOfType() SymbolicValue {
	return &ExecutedStep{}
}

func (r *ExecutedStep) GetGoMethod(name string) (*GoFunction, bool) {
	return nil, false
}

func (r *ExecutedStep) Prop(name string) SymbolicValue {
	switch name {
	case "result":
		return ANY
	case "end_time":
		return &Date{}
	}

	method, ok := r.GetGoMethod(name)
	if !ok {
		panic(FormatErrPropertyDoesNotExist(name, r))
	}
	return method
}

func (*ExecutedStep) PropertyNames() []string {
	return EXECUTED_STEP_PROPNAMES
}

func (routine *ExecutedStep) WaitResult(ctx *Context) (SymbolicValue, *Error) {
	return ANY, nil
}

func (routine *ExecutedStep) Cancel(*Context) {
}

func (r *ExecutedStep) Widen() (SymbolicValue, bool) {
	return nil, false
}

func (r *ExecutedStep) IsWidenable() bool {
	return false
}

func (r *ExecutedStep) String() string {
	return "executed-step"
}
