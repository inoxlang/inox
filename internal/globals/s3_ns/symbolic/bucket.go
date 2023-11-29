package s3_ns

import (
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/prettyprint"
	pprint "github.com/inoxlang/inox/internal/prettyprint"
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

func (r *Bucket) PrettyPrint(w prettyprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("s3-bucket")
}

func (r *Bucket) WidestOfType() symbolic.Value {
	return &Bucket{}
}
