package symbolic

import (
	"bufio"
	"strconv"

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
	value    float64
	hasValue bool
}

func (f *Float) Test(v SymbolicValue) bool {
	otherFloat, ok := v.(*Float)
	if !ok {
		return false
	}
	if !f.hasValue {
		return true
	}
	return otherFloat.hasValue && f.value == otherFloat.value
}

func NewFloat(v float64) *Float {
	return &Float{
		value:    v,
		hasValue: true,
	}
}

func (f *Float) IsWidenable() bool {
	return f.hasValue
}

func (f *Float) Widen() (SymbolicValue, bool) {
	if f.hasValue {
		return ANY_FLOAT, true
	}
	return nil, false
}

func (f *Float) Static() Pattern {
	return &TypePattern{val: ANY_FLOAT}
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
	value    int64
	hasValue bool
}

func NewInt(v int64) *Int {
	return &Int{
		value:    v,
		hasValue: true,
	}
}

func (i *Int) Test(v SymbolicValue) bool {
	otherInt, ok := v.(*Int)
	if !ok {
		return false
	}
	if !i.hasValue {
		return true
	}
	return otherInt.hasValue && i.value == otherInt.value
}

func (i *Int) Widen() (SymbolicValue, bool) {
	if i.hasValue {
		return ANY_INT, true
	}
	return nil, false
}

func (i *Int) IsWidenable() bool {
	return i.hasValue
}

func (i *Int) Static() Pattern {
	return &TypePattern{val: ANY_INT}
}

func (i *Int) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%int")))
	if i.hasValue {
		utils.PanicIfErr(w.WriteByte('('))
		utils.Must(w.Write(utils.StringAsBytes(strconv.FormatInt(i.value, 10))))
		utils.PanicIfErr(w.WriteByte(')'))
	}
}

func (i *Int) WidestOfType() SymbolicValue {
	return ANY_INT
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
