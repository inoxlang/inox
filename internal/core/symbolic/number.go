package symbolic

import (
	"strconv"

	pprint "github.com/inoxlang/inox/internal/pretty_print"
)

var (
	_            = []Integral{(*Int)(nil), (*Byte)(nil), (*AnyIntegral)(nil)}
	ANY_INTEGRAL = &AnyIntegral{}
	ANY_INT      = &Int{}
	ANY_FLOAT    = &Float{}
	INT_1        = NewInt(1)
	INT_2        = NewInt(2)
	INT_1_OR_2   = NewMultivalue(INT_1, INT_2)
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

func NewFloat(v float64) *Float {
	return &Float{
		value:    v,
		hasValue: true,
	}
}

func (f *Float) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	otherFloat, ok := v.(*Float)
	if !ok {
		return false
	}
	if !f.hasValue {
		return true
	}
	return otherFloat.hasValue && f.value == otherFloat.value
}

func (f *Float) IsConcretizable() bool {
	return f.hasValue
}

func (f *Float) Concretize(ctx ConcreteContext) any {
	if !f.IsConcretizable() {
		panic(ErrNotConcretizable)
	}
	return extData.ConcreteValueFactories.CreateFloat(f.value)
}

func (f *Float) Static() Pattern {
	return &TypePattern{val: ANY_FLOAT}
}

func (f *Float) PrettyPrint(w PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("float")
	if f.hasValue {
		w.WriteByte('(')
		w.WriteString(strconv.FormatFloat(f.value, 'g', -1, 64))
		w.WriteByte(')')
	}
}

func (f *Float) WidestOfType() Value {
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

func (i *Int) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	otherInt, ok := v.(*Int)
	if !ok {
		return false
	}
	if !i.hasValue {
		return true
	}
	return otherInt.hasValue && i.value == otherInt.value
}

func (i *Int) IsConcretizable() bool {
	return i.hasValue
}

func (i *Int) Concretize(ctx ConcreteContext) any {
	if !i.IsConcretizable() {
		panic(ErrNotConcretizable)
	}
	return extData.ConcreteValueFactories.CreateInt(i.value)
}

func (i *Int) Static() Pattern {
	return &TypePattern{val: ANY_INT}
}

func (i *Int) PrettyPrint(w PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("int")
	if i.hasValue {
		w.WriteByte('(')
		w.WriteString(strconv.FormatInt(i.value, 10))
		w.WriteByte(')')
	}
}

func (i *Int) WidestOfType() Value {
	return ANY_INT
}

func (i *Int) Int64() (n *Int, signed bool) {
	return i, true
}

// An AnyIntegral represents a symbolic Integral we do not know the concrete type.
type AnyIntegral struct {
	_ int
}

func (*AnyIntegral) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	_, ok := v.(Integral)

	return ok
}

func (*AnyIntegral) PrettyPrint(w PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("integral")
}

func (*AnyIntegral) WidestOfType() Value {
	return ANY_INTEGRAL
}

func (*AnyIntegral) IteratorElementKey() Value {
	return ANY
}

func (*AnyIntegral) IteratorElementValue() Value {
	return ANY
}

func (*AnyIntegral) Int64() (i *Int, signed bool) {
	return ANY_INT, true
}
