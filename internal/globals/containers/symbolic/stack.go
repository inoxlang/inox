package containers

import (
	"bufio"

	"github.com/inoxlang/inox/internal/core/symbolic"
	pprint "github.com/inoxlang/inox/internal/pretty_print"

	"github.com/inoxlang/inox/internal/utils"
)

var _ = []symbolic.Iterable{&Stack{}}

type Stack struct {
	symbolic.UnassignablePropsMixin
	_ int
}

func (*Stack) Test(v symbolic.SymbolicValue) bool {
	_, ok := v.(*Stack)
	return ok
}

func (s *Stack) GetGoMethod(name string) (*symbolic.GoFunction, bool) {
	switch name {
	case "push":
		return symbolic.WrapGoMethod(s.Push), true
	case "pop":
		return symbolic.WrapGoMethod(s.Pop), true
	case "peek":
		return symbolic.WrapGoMethod(s.Peek), true
	}
	return nil, false
}

func (s *Stack) Prop(name string) symbolic.SymbolicValue {
	return symbolic.GetGoMethodOrPanic(name, s)
}

func (*Stack) PropertyNames() []string {
	return []string{"push", "pop", "peek"}
}

func (*Stack) Push(ctx *symbolic.Context, elems ...symbolic.SymbolicValue) {

}

func (*Stack) Pop(ctx *symbolic.Context) {

}

func (*Stack) Peek(ctx *symbolic.Context) symbolic.SymbolicValue {
	return &symbolic.Any{}
}

func (*Stack) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%set")))
	return
}

func (*Stack) IteratorElementKey() symbolic.SymbolicValue {
	return &symbolic.Any{}
}

func (*Stack) IteratorElementValue() symbolic.SymbolicValue {
	return &symbolic.Any{}
}

func (*Stack) WidestOfType() symbolic.SymbolicValue {
	return &Stack{}
}
