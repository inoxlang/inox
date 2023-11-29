package containers

import (
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/prettyprint"
	pprint "github.com/inoxlang/inox/internal/prettyprint"
)

var _ = []symbolic.Iterable{(*Queue)(nil)}

type Queue struct {
	symbolic.UnassignablePropsMixin
	_ int
}

func (*Queue) Test(v symbolic.Value, state symbolic.RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	_, ok := v.(*Queue)
	return ok
}

func (q *Queue) GetGoMethod(name string) (*symbolic.GoFunction, bool) {
	switch name {
	case "enqueue":
		return symbolic.WrapGoMethod(q.Enqueue), true
	case "dequeue":
		return symbolic.WrapGoMethod(q.Dequeue), true
	case "peek":
		return symbolic.WrapGoMethod(q.Peek), true
	}
	return nil, false
}

func (q *Queue) Prop(name string) symbolic.Value {
	return symbolic.GetGoMethodOrPanic(name, q)
}

func (*Queue) PropertyNames() []string {
	return []string{"enqueue", "dequeue", "peek"}
}

func (*Queue) Enqueue(ctx *symbolic.Context, elems symbolic.Value) {

}

func (*Queue) Dequeue(ctx *symbolic.Context) (symbolic.Value, *symbolic.Bool) {
	return &symbolic.Any{}, nil
}

func (*Queue) Peek(ctx *symbolic.Context) (symbolic.Value, *symbolic.Bool) {
	return &symbolic.Any{}, nil
}

func (*Queue) PrettyPrint(w prettyprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("queue")
	return
}

func (*Queue) IteratorElementKey() symbolic.Value {
	return &symbolic.Any{}
}

func (*Queue) IteratorElementValue() symbolic.Value {
	return &symbolic.Any{}
}

func (*Queue) WidestOfType() symbolic.Value {
	return &Queue{}
}
