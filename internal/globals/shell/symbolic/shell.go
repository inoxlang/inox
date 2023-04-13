package symbolic

import (
	symbolic "github.com/inox-project/inox/internal/core/symbolic"
)

var (
	SHELL_PROPNAMES = []string{"start", "stop"}

	_ = []symbolic.Readable{&Shell{}}
	_ = []symbolic.Writable{&Shell{}}
)

// A Shell represents a symbolic Shell.
type Shell struct {
	symbolic.UnassignablePropsMixin
	_ int
}

func (r *Shell) Test(v symbolic.SymbolicValue) bool {
	switch v.(type) {
	case *Shell:
		return true
	default:
		return false
	}
}

func (r *Shell) Clone(clones map[uintptr]symbolic.SymbolicValue) symbolic.SymbolicValue {
	return &Shell{}
}

func (r *Shell) WidestOfType() symbolic.SymbolicValue {
	return &Shell{}
}

func (r *Shell) GetGoMethod(name string) (*symbolic.GoFunction, bool) {
	switch name {
	case "start":
		return symbolic.WrapGoClosure(func(ctx *symbolic.Context) {}), true
	case "stop":
		return symbolic.WrapGoClosure(func(ctx *symbolic.Context) {}), true
	}
	return &symbolic.GoFunction{}, false
}

func (r *Shell) Prop(name string) symbolic.SymbolicValue {
	method, ok := r.GetGoMethod(name)
	if !ok {
		panic(symbolic.FormatErrPropertyDoesNotExist(name, r))
	}
	return method
}

func (*Shell) PropertyNames() []string {
	return SHELL_PROPNAMES
}

func (r *Shell) Reader() *symbolic.Reader {
	return &symbolic.Reader{}
}

func (r *Shell) Writer() *symbolic.Writer {
	return &symbolic.Writer{}
}

func (r *Shell) Widen() (symbolic.SymbolicValue, bool) {
	return nil, false
}

func (r *Shell) IsWidenable() bool {
	return false
}

func (r *Shell) IsMutable() bool {
	return true
}

func (r *Shell) String() string {
	return "%inox-shell"
}
