package symbolic

import (
	"bufio"

	pprint "github.com/inoxlang/inox/internal/pretty_print"
	"github.com/inoxlang/inox/internal/utils"
)

var (
	_            = []Integral{(*Int)(nil), (*Byte)(nil), (*AnyIntegral)(nil)}
	ANY_INTEGRAL = &AnyIntegral{}
	ANY_INT      = &Int{}
	ANY_FLOAT    = &Float{}
)

type Integral interface {
	Int64() (i *Int, signed bool)
}

// A Float represents a symbolic Float.
type Float struct {
	SerializableMixin
}

func (f *Float) Test(v SymbolicValue) bool {
	_, ok := v.(*Float)
	return ok
}

func (f *Float) IsWidenable() bool {
	return false
}

func (f *Float) Widen() (SymbolicValue, bool) {
	return nil, false
}

func (f *Float) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%float")))
}

func (f *Float) WidestOfType() SymbolicValue {
	return &Float{}
}

// An Int represents a symbolic Int.
type Int struct {
	SerializableMixin
}

func (i *Int) Test(v SymbolicValue) bool {
	_, ok := v.(*Int)
	return ok
}

func (a *Int) Widen() (SymbolicValue, bool) {
	return nil, false
}

func (i *Int) IsWidenable() bool {
	return false
}

func (i *Int) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%int")))
	return
}

func (i *Int) WidestOfType() SymbolicValue {
	return &Int{}
}

func (i *Int) Int64() (n *Int, signed bool) {
	return i, true
}

// An AnyIntegral represents a symbolic Integral we do not know the concrete type.
type AnyIntegral struct {
	_ int
}

func (*AnyIntegral) Test(v SymbolicValue) bool {
	_, ok := v.(Integral)

	return ok
}

func (*AnyIntegral) Widen() (SymbolicValue, bool) {
	return nil, false
}

func (*AnyIntegral) IsWidenable() bool {
	return false
}

func (*AnyIntegral) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%integral")))
}

func (*AnyIntegral) WidestOfType() SymbolicValue {
	return ANY_INTEGRAL
}

func (*AnyIntegral) IteratorElementKey() SymbolicValue {
	return ANY
}

func (*AnyIntegral) IteratorElementValue() SymbolicValue {
	return ANY
}

func (*AnyIntegral) Int64() (i *Int, signed bool) {
	return &Int{}, true
}
