package s3_ns

import (
	"bufio"

	"github.com/inoxlang/inox/internal/core/symbolic"
	pprint "github.com/inoxlang/inox/internal/pretty_print"

	"github.com/inoxlang/inox/internal/utils"
)

type Bucket struct {
	symbolic.UnassignablePropsMixin
	_ int
}

func (r *Bucket) Test(v symbolic.SymbolicValue) bool {
	_, ok := v.(*Bucket)
	return ok
}

func (r Bucket) Clone(clones map[uintptr]symbolic.SymbolicValue) symbolic.SymbolicValue {
	return &Bucket{}
}

func (serv *Bucket) GetGoMethod(name string) (*symbolic.GoFunction, bool) {
	return nil, false
}

func (b *Bucket) Prop(name string) symbolic.SymbolicValue {
	return symbolic.GetGoMethodOrPanic(name, b)
}

func (*Bucket) PropertyNames() []string {
	return nil
}

func (r *Bucket) Widen() (symbolic.SymbolicValue, bool) {
	return nil, false
}

func (a *Bucket) IsWidenable() bool {
	return false
}

func (r *Bucket) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%s3-bucket")))
	return
}

func (r *Bucket) WidestOfType() symbolic.SymbolicValue {
	return &Bucket{}
}
