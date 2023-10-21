package symbolic

import (
	"github.com/inoxlang/inox/internal/core/symbolic"
	pprint "github.com/inoxlang/inox/internal/pretty_print"
)

var (
	SHELL_PROPNAMES = []string{"start", "stop"}

	_ = []symbolic.Readable{(*Shell)(nil)}
	_ = []symbolic.Writable{(*Shell)(nil)}
)

// A Shell represents a symbolic Shell.
type Shell struct {
	symbolic.UnassignablePropsMixin
	_ int
}

func (r *Shell) Test(v symbolic.Value, state symbolic.RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	switch v.(type) {
	case *Shell:
		return true
	default:
		return false
	}
}

func (r *Shell) WidestOfType() symbolic.Value {
	return &Shell{}
}

func (r *Shell) GetGoMethod(name string) (*symbolic.GoFunction, bool) {
	switch name {
	case "start":
		return symbolic.WrapGoClosure(func(ctx *symbolic.Context) {}), true
	case "stop":
		return symbolic.WrapGoClosure(func(ctx *symbolic.Context) {}), true
	}
	return nil, false
}

func (r *Shell) Prop(name string) symbolic.Value {
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

func (r *Shell) IsMutable() bool {
	return true
}

func (r *Shell) PrettyPrint(w symbolic.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("inox-shell")
}
