package s3_ns

import (
	"github.com/inoxlang/inox/internal/core/symbolic"
	pprint "github.com/inoxlang/inox/internal/pretty_print"
)

type Bucket struct {
	symbolic.UnassignablePropsMixin
	_ int
}

func (r *Bucket) Test(v symbolic.Value, state symbolic.RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	_, ok := v.(*Bucket)
	return ok
}

func (serv *Bucket) GetGoMethod(name string) (*symbolic.GoFunction, bool) {
	return nil, false
}

func (b *Bucket) Prop(name string) symbolic.Value {
	return symbolic.GetGoMethodOrPanic(name, b)
}

func (*Bucket) PropertyNames() []string {
	return nil
}

func (r *Bucket) PrettyPrint(w symbolic.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("s3-bucket")
}

func (r *Bucket) WidestOfType() symbolic.Value {
	return &Bucket{}
}
