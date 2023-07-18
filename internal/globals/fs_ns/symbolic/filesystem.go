package fs_ns

import (
	"bufio"

	"github.com/inoxlang/inox/internal/core/symbolic"
	pprint "github.com/inoxlang/inox/internal/pretty_print"

	"github.com/inoxlang/inox/internal/utils"
)

var (
	ANY_FILESYSTEM = &Filesystem{}
)

type Filesystem struct {
	symbolic.UnassignablePropsMixin
	_ int
}

func (fls *Filesystem) Test(v symbolic.SymbolicValue) bool {
	_, ok := v.(*Filesystem)
	return ok
}

func (fls *Filesystem) GetGoMethod(name string) (*symbolic.GoFunction, bool) {
	return nil, false
}

func (fls *Filesystem) Prop(name string) symbolic.SymbolicValue {
	method, ok := fls.GetGoMethod(name)
	if !ok {
		panic(symbolic.FormatErrPropertyDoesNotExist(name, fls))
	}
	return method
}

func (fls *Filesystem) Widen() (symbolic.SymbolicValue, bool) {
	return nil, false
}

func (fls *Filesystem) IsWidenable() bool {
	return false
}

func (fls *Filesystem) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%filesystem")))
}

func (fls *Filesystem) WidestOfType() symbolic.SymbolicValue {
	return ANY_FILESYSTEM
}
