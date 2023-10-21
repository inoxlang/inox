package fs_ns

import (
	"github.com/inoxlang/inox/internal/core/symbolic"
	pprint "github.com/inoxlang/inox/internal/pretty_print"
)

var (
	ANY_FILESYSTEM = &Filesystem{}
)

type Filesystem struct {
	symbolic.UnassignablePropsMixin
	_ int
}

func (fls *Filesystem) Test(v symbolic.Value, state symbolic.RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	_, ok := v.(*Filesystem)
	return ok
}

func (fls *Filesystem) GetGoMethod(name string) (*symbolic.GoFunction, bool) {
	return nil, false
}

func (fls *Filesystem) Prop(name string) symbolic.Value {
	method, ok := fls.GetGoMethod(name)
	if !ok {
		panic(symbolic.FormatErrPropertyDoesNotExist(name, fls))
	}
	return method
}

func (fls *Filesystem) PrettyPrint(w symbolic.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("filesystem")
}

func (fls *Filesystem) WidestOfType() symbolic.Value {
	return ANY_FILESYSTEM
}
