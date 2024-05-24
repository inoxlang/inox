package symbolic

import (
	pprint "github.com/inoxlang/inox/internal/prettyprint"
)

const (
	LTHREAD_META_GROUP_SECTION   = "group"
	LTHREAD_META_ALLOW_SECTION   = "allow"
	LTHREAD_META_GLOBALS_SECTION = "globals"
)

var (
	ROUTINE_PROPNAMES       = []string{"wait_result", "cancel", "steps"}
	ROUTINE_GROUP_PROPNAMES = []string{"wait_results", "cancel_all"}
	EXECUTED_STEP_PROPNAMES = []string{"result", "end_time"}
	LTHREAD_SECTION_NAMES   = []string{LTHREAD_META_ALLOW_SECTION, LTHREAD_META_GLOBALS_SECTION, LTHREAD_META_GROUP_SECTION}

	ANY_LTHREAD       = &LThread{}
	ANY_LTHREAD_GROUP = &LThreadGroup{}
	ANY_EXECUTED_STEP = &ExecutedStep{}
)

// A LThread represents a symbolic LThread.
type LThread struct {
	UnassignablePropsMixin
	_ int
}

func (t *LThread) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	switch v.(type) {
	case *LThread:
		return true
	default:
		return false
	}
}

func (t *LThread) WidestOfType() Value {
	return ANY_LTHREAD
}

func (t *LThread) GetGoMethod(name string) (*GoFunction, bool) {
	switch name {
	case "wait_result":
		return WrapGoMethod(t.WaitResult), true
	case "cancel":
		return WrapGoMethod(t.Cancel), true
	}
	return nil, false
}

func (t *LThread) Prop(name string) Value {
	switch name {
	case "steps":
		return NewArrayOf(&ExecutedStep{})
	}
	method, ok := t.GetGoMethod(name)
	if !ok {
		panic(FormatErrPropertyDoesNotExist(name, t))
	}
	return method
}

func (*LThread) PropertyNames() []string {
	return ROUTINE_PROPNAMES
}

func (t *LThread) WaitResult(ctx *Context) (Value, *Error) {
	return ANY, nil
}

func (t *LThread) Cancel(*Context) {

}

func (t *LThread) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("lthread")
}

// A LThreadGroup represents a symbolic LThreadGroup.
type LThreadGroup struct {
	UnassignablePropsMixin
	_ int
}

func (g *LThreadGroup) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	switch v.(type) {
	case *LThreadGroup:
		return true
	default:
		return false
	}
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

func (g *LThreadGroup) Prop(name string) Value {
	method, ok := g.GetGoMethod(name)
	if !ok {
		panic(FormatErrPropertyDoesNotExist(name, g))
	}
	return method
}

func (*LThreadGroup) PropertyNames() []string {
	return ROUTINE_GROUP_PROPNAMES
}

func (g *LThreadGroup) Add(newRt *LThread) {

}

func (g *LThreadGroup) WaitAllResults(ctx *Context) (*Array, *Error) {
	return ANY_ARRAY, nil
}

func (g *LThreadGroup) CancelAll(*Context) {

}

func (g *LThreadGroup) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("lthread-group")
}

func (g *LThreadGroup) WidestOfType() Value {
	return ANY_LTHREAD_GROUP
}

// An ExecutedStep represents a symbolic ExecutedStep.
type ExecutedStep struct {
	UnassignablePropsMixin
	_ int
}

func (s *ExecutedStep) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	switch v.(type) {
	case *ExecutedStep:
		return true
	default:
		return false
	}
}

func (s *ExecutedStep) WidestOfType() Value {
	return ANY_EXECUTED_STEP
}

func (s *ExecutedStep) GetGoMethod(name string) (*GoFunction, bool) {
	return nil, false
}

func (s *ExecutedStep) Prop(name string) Value {
	switch name {
	case "result":
		return ANY
	case "end_time":
		return ANY_DATETIME
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

func (s *ExecutedStep) WaitResult(ctx *Context) (Value, *Error) {
	return ANY, nil
}

func (s *ExecutedStep) Cancel(*Context) {
}

func (s *ExecutedStep) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("executed-step")
}
