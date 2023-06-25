package containers

import (
	"bufio"

	"github.com/inoxlang/inox/internal/core/symbolic"
	pprint "github.com/inoxlang/inox/internal/pretty_print"

	"github.com/inoxlang/inox/internal/utils"
)

var _ = []symbolic.Iterable{&Ranking{}, &Rank{}}

type Ranking struct {
	symbolic.UnassignablePropsMixin
	_ int
}

func (r *Ranking) Test(v symbolic.SymbolicValue) bool {
	_, ok := v.(*Ranking)
	return ok
}

func (r *Ranking) Clone(clones map[uintptr]symbolic.SymbolicValue) symbolic.SymbolicValue {
	return &Ranking{}
}

func (r *Ranking) GetGoMethod(name string) (*symbolic.GoFunction, bool) {
	switch name {
	case "add":
		return symbolic.WrapGoMethod(r.Add), true
	case "remove":
		return symbolic.WrapGoMethod(r.Remove), true
	}
	return nil, false
}

func (r *Ranking) Prop(name string) symbolic.SymbolicValue {
	return symbolic.GetGoMethodOrPanic(name, r)
}

func (*Ranking) PropertyNames() []string {
	return []string{"add", "remove"}
}

func (f *Ranking) Add(ctx *symbolic.Context, v symbolic.SymbolicValue, score *symbolic.Float) {

}

func (f *Ranking) Remove(ctx *symbolic.Context, v symbolic.SymbolicValue) {

}

func (r *Ranking) Widen() (symbolic.SymbolicValue, bool) {
	return nil, false
}

func (a *Ranking) IsWidenable() bool {
	return false
}

func (r *Ranking) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%ranking")))
	return
}

func (r *Ranking) IteratorElementKey() symbolic.SymbolicValue {
	return &symbolic.Any{}
}

func (r *Ranking) IteratorElementValue() symbolic.SymbolicValue {
	return &symbolic.Any{}
}

func (r *Ranking) WidestOfType() symbolic.SymbolicValue {
	return &Ranking{}
}

type Rank struct {
	symbolic.UnassignablePropsMixin
	_ int
}

func (r *Rank) Test(v symbolic.SymbolicValue) bool {
	_, ok := v.(*Rank)
	return ok
}

func (r *Rank) Clone(clones map[uintptr]symbolic.SymbolicValue) symbolic.SymbolicValue {
	return &Rank{}
}

func (r *Rank) GetGoMethod(name string) (*symbolic.GoFunction, bool) {
	return nil, false
}

func (r *Rank) Prop(name string) symbolic.SymbolicValue {
	switch name {
	case "values":
		return symbolic.NewListOf(&symbolic.Any{})
	}
	return symbolic.GetGoMethodOrPanic(name, r)
}

func (*Rank) PropertyNames() []string {
	return []string{"values"}
}

func (r *Rank) Widen() (symbolic.SymbolicValue, bool) {
	return nil, false
}

func (a *Rank) IsWidenable() bool {
	return false
}

func (r *Rank) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%rank")))
	return
}

func (r *Rank) IteratorElementKey() symbolic.SymbolicValue {
	return &symbolic.Any{}
}

func (r *Rank) IteratorElementValue() symbolic.SymbolicValue {
	return &symbolic.Any{}
}

func (r *Rank) WidestOfType() symbolic.SymbolicValue {
	return &Rank{}
}
