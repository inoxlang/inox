package containers

import (
	"bufio"

	symbolic "github.com/inoxlang/inox/internal/core/symbolic"
	pprint "github.com/inoxlang/inox/internal/pretty_print"

	"github.com/inoxlang/inox/internal/utils"
)

var _ = []symbolic.Iterable{&Map{}}

type Map struct {
	symbolic.UnassignablePropsMixin
	_ int
}

func (*Map) Test(v symbolic.SymbolicValue) bool {
	_, ok := v.(*Map)
	return ok
}

func (r Map) Clone(clones map[uintptr]symbolic.SymbolicValue) symbolic.SymbolicValue {
	return &Map{}
}

func (m *Map) GetGoMethod(name string) (*symbolic.GoFunction, bool) {
	switch name {
	case "insert":
		return symbolic.WrapGoMethod(m.Insert), true
	case "update":
		return symbolic.WrapGoMethod(m.Update), true
	case "remove":
		return symbolic.WrapGoMethod(m.Remove), true
	case "get":
		return symbolic.WrapGoMethod(m.Get), true
	}
	return nil, false
}

func (m *Map) Prop(name string) symbolic.SymbolicValue {
	return symbolic.GetGoMethodOrPanic(name, m)
}

func (*Map) PropertyNames() []string {
	return []string{"insert", "update", "remove", "get"}
}

func (*Map) Insert(ctx *symbolic.Context, k, v symbolic.SymbolicValue) {

}

func (*Map) Update(ctx *symbolic.Context, k, v symbolic.SymbolicValue) {

}

func (*Map) Remove(ctx *symbolic.Context, k symbolic.SymbolicValue) {

}

func (*Map) Get(ctx *symbolic.Context, k symbolic.SymbolicValue) symbolic.SymbolicValue {
	return &symbolic.Any{}
}

func (*Map) Widen() (symbolic.SymbolicValue, bool) {
	return nil, false
}

func (a *Map) IsWidenable() bool {
	return false
}

func (*Map) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%map")))
	return
}

func (m *Map) IteratorElementKey() symbolic.SymbolicValue {
	return &symbolic.Any{}
}

func (*Map) IteratorElementValue() symbolic.SymbolicValue {
	return &symbolic.Any{}
}

func (*Map) WidestOfType() symbolic.SymbolicValue {
	return &Map{}
}
