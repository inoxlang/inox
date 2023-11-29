package symbolic

import (
	"math"
	"strconv"
	"strings"

	pprint "github.com/inoxlang/inox/internal/prettyprint"
)

var (
	_            = []Integral{(*Int)(nil), (*Byte)(nil), (*AnyIntegral)(nil)}
	ANY_INTEGRAL = &AnyIntegral{}

	ANY_INT    = &Int{}
	INT_0      = NewInt(0)
	INT_1      = NewInt(1)
	INT_2      = NewInt(2)
	INT_3      = NewInt(3)
	INT_1_OR_2 = NewMultivalue(INT_1, INT_2)
	MAX_INT    = NewInt(math.MaxInt64)

	ANY_FLOAT = &Float{}
	MAX_FLOAT = NewFloat(math.MaxFloat64)
	FLOAT_0   = NewFloat(0)
	FLOAT_1   = NewFloat(1)
	FLOAT_2   = NewFloat(2)
	FLOAT_3   = NewFloat(3)
)

type Integral interface {
	Int64() (i *Int, signed bool)
}

// A Float represents a symbolic Float.
type Float struct {
	SerializableMixin
	value    float64
	hasValue bool

	//this field can be set whatever the value of hasValue.
	matchingPattern *FloatRangePattern
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
		if f.matchingPattern == nil || f.matchingPattern.TestValue(otherFloat, state) {
			return true
		}
		return otherFloat.matchingPattern != nil && f.matchingPattern.Test(otherFloat.matchingPattern, state)
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

func (f *Float) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("float")
	if f.hasValue {
		w.WriteByte('(')
		s := strconv.FormatFloat(f.value, 'g', -1, 64)
		w.WriteString(s)
		if !strings.ContainsAny(s, ".e") {
			w.WriteString(".0")
		}
		w.WriteByte(')')
	} else if f.matchingPattern != nil {
		w.WriteByte('(')
		f.matchingPattern.floatRange.PrettyPrint(w.WithDepthIndent(w.Depth+1, 0), config)
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

	//this field can be set whatever the value of hasValue.
	matchingPattern *IntRangePattern
}

func NewInt(v int64) *Int {
	return &Int{
		value:    v,
		hasValue: true,
	}
}

func (i *Int) WithMatchingPattern(pattern *IntRangePattern) *Int {
	if i.matchingPattern == pattern {
		return i
	}
	return &Int{
		value:           i.value,
		hasValue:        i.hasValue,
		matchingPattern: pattern,
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
		if i.matchingPattern == nil || i.matchingPattern.TestValue(otherInt, state) {
			return true
		}
		return otherInt.matchingPattern != nil && i.matchingPattern.Test(otherInt.matchingPattern, state)
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

func (i *Int) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("int")
	if i.hasValue {
		w.WriteByte('(')
		w.WriteString(strconv.FormatInt(i.value, 10))
		w.WriteByte(')')
	} else if i.matchingPattern != nil {
		w.WriteByte('(')
		i.matchingPattern.intRange.PrettyPrint(w.WithDepthIndent(w.Depth+1, 0), config)
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

func (*AnyIntegral) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
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
