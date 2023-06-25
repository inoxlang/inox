package containers

import (
	"bufio"

	"github.com/inoxlang/inox/internal/core/symbolic"
	pprint "github.com/inoxlang/inox/internal/pretty_print"

	"github.com/inoxlang/inox/internal/utils"
)

var _ = []symbolic.Iterable{&Queue{}}

type Queue struct {
	symbolic.UnassignablePropsMixin
	_ int
}

func (*Queue) Test(v symbolic.SymbolicValue) bool {
	_, ok := v.(*Queue)
	return ok
}

func (r Queue) Clone(clones map[uintptr]symbolic.SymbolicValue) symbolic.SymbolicValue {
	return &Queue{}
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

func (q *Queue) Prop(name string) symbolic.SymbolicValue {
	return symbolic.GetGoMethodOrPanic(name, q)
}

func (*Queue) PropertyNames() []string {
	return []string{"enqueue", "dequeue", "peek"}
}

func (*Queue) Enqueue(ctx *symbolic.Context, elems symbolic.SymbolicValue) {

}

func (*Queue) Dequeue(ctx *symbolic.Context) (symbolic.SymbolicValue, *symbolic.Bool) {
	return &symbolic.Any{}, nil
}

func (*Queue) Peek(ctx *symbolic.Context) (symbolic.SymbolicValue, *symbolic.Bool) {
	return &symbolic.Any{}, nil
}

func (*Queue) Widen() (symbolic.SymbolicValue, bool) {
	return nil, false
}

func (*Queue) IsWidenable() bool {
	return false
}

func (*Queue) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%queue")))
	return
}

func (*Queue) IteratorElementKey() symbolic.SymbolicValue {
	return &symbolic.Any{}
}

func (*Queue) IteratorElementValue() symbolic.SymbolicValue {
	return &symbolic.Any{}
}

func (*Queue) WidestOfType() symbolic.SymbolicValue {
	return &Queue{}
}
