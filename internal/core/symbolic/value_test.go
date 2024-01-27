package symbolic

import (
	"testing"

	"github.com/inoxlang/inox/internal/parse"
)

func TestSymbolicAny(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		any := ANY

		assertTest(t, any, any)
		assertTest(t, any, &Int{})
	})

}

func TestSymbolicNil(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		_nil := &NilT{}

		assertTest(t, _nil, _nil)
		assertTestFalse(t, _nil, &Int{})
	})

}

func TestSymbolicBool(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		bool := ANY_BOOL

		assertTest(t, bool, bool)
		assertTest(t, bool, ANY_BOOL)
		assertTestFalse(t, bool, &Int{})
	})

}

func TestSymbolicFloat(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		floatMatchingSpecificPattern := &Float{matchingPattern: NewFloatRangePattern(NewIncludedEndFloatRange(FLOAT_1, FLOAT_2))}

		assertTest(t, ANY_FLOAT, ANY_FLOAT)
		assertTest(t, ANY_FLOAT, floatMatchingSpecificPattern)
		assertTest(t, ANY_FLOAT, FLOAT_1)

		//check FLOAT_1
		assertTest(t, FLOAT_1, FLOAT_1)
		assertTestFalse(t, FLOAT_1, ANY_FLOAT)
		assertTestFalse(t, FLOAT_1, FLOAT_2)
		assertTestFalse(t, FLOAT_1, floatMatchingSpecificPattern)

		//check floatMatchingSpecificPattern
		assertTest(t, floatMatchingSpecificPattern, floatMatchingSpecificPattern)
		assertTest(t, floatMatchingSpecificPattern, FLOAT_1)
		assertTest(t, floatMatchingSpecificPattern, FLOAT_2)
		assertTestFalse(t, floatMatchingSpecificPattern, ANY_FLOAT)
	})

}

func TestSymbolicInt(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		anyInt := &Int{}
		anyIntMatchingSpecificPattern := &Int{matchingPattern: &IntRangePattern{
			intRange: NewIncludedEndIntRange(INT_1, INT_2),
		}}

		assertTest(t, anyInt, anyInt)
		assertTest(t, anyInt, &Int{})
		assertTest(t, anyInt, INT_1)
		assertTest(t, anyInt, anyIntMatchingSpecificPattern)
		assertTestFalse(t, anyInt, ANY_FLOAT)
	})

}

func TestSymbolicRune(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		rune := &Rune{}

		assertTest(t, rune, rune)
		assertTest(t, rune, &Rune{})
		assertTestFalse(t, rune, &Int{})
	})

}

func TestSymbolicIdentifier(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		specificIdent := &Identifier{name: "foo"}
		ident := &Identifier{}

		assertTest(t, specificIdent, specificIdent)
		assertTestFalse(t, specificIdent, ident)

		assertTest(t, ident, ident)
		assertTest(t, ident, specificIdent)
	})

}

func TestSymbolicOption(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		option := NewOption("a", NewInt(1))

		assertTest(t, option, NewOption("a", NewInt(1)))
		assertTestFalse(t, option, NewOption("a", NewInt(2)))
		assertTestFalse(t, option, NewOption("b", NewInt(1)))
		assertTestFalse(t, option, &String{})
		assertTestFalse(t, option, &Int{})

		assertTest(t, ANY_OPTION, ANY_OPTION)
		assertTest(t, ANY_OPTION, NewOption("a", NewInt(1)))
		assertTest(t, ANY_OPTION, NewOption("a", NewInt(2)))
		assertTest(t, ANY_OPTION, NewOption("b", NewInt(1)))
		assertTestFalse(t, ANY_OPTION, &String{})
		assertTestFalse(t, ANY_OPTION, &Int{})
	})

}

func TestSymbolicNode(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		specificNode := &AstNode{Node: &parse.ContinueStatement{}}
		node := &AstNode{}

		assertTest(t, specificNode, specificNode)
		assertTestFalse(t, specificNode, node)

		assertTest(t, node, node)
		assertTest(t, node, specificNode)
	})

}

func TestSymbolicError(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		err := &Error{data: &Int{}}

		assertTest(t, err, err)
		assertTest(t, err, &Error{data: &Int{}})
		assertTestFalse(t, err, &Int{})
	})

}

func TestSymbolicGoFunction(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		{
			anyFunc := &GoFunction{}
			assertTest(t, anyFunc, anyFunc)
			assertTest(t, anyFunc, &GoFunction{})
			assertTestFalse(t, anyFunc, &Int{})
		}

		{
			specificFunc := &GoFunction{fn: symbolicGoFn}
			assertTest(t, specificFunc, specificFunc)
			assertTestFalse(t, specificFunc, &GoFunction{})
			assertTestFalse(t, specificFunc, &Int{})
		}
	})

}

func TestSymbolicRuneSlice(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		slice := &RuneSlice{}

		assertTest(t, slice, slice)
		assertTest(t, slice, &RuneSlice{})
		assertTestFalse(t, slice, &String{})
		assertTestFalse(t, slice, &Int{})
	})

}

func TestSymbolicQuantityRange(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		anyQtyRange := &QuantityRange{element: ANY_SERIALIZABLE}

		assertTest(t, anyQtyRange, anyQtyRange)
		assertTest(t, anyQtyRange, &QuantityRange{element: ANY_SERIALIZABLE})
		assertTest(t, anyQtyRange, NewQuantityRange(ANY_BYTECOUNT))
		assertTestFalse(t, anyQtyRange, ANY_STRING)
		assertTestFalse(t, anyQtyRange, ANY_INT)

		qtyRange := NewQuantityRange(ANY_BYTECOUNT)

		assertTest(t, qtyRange, qtyRange)
		assertTest(t, qtyRange, NewQuantityRange(ANY_BYTECOUNT))
		assertTestFalse(t, qtyRange, &QuantityRange{element: ANY_SERIALIZABLE})
		assertTestFalse(t, qtyRange, ANY_STRING)
		assertTestFalse(t, qtyRange, ANY_INT)
	})

}

func TestSymbolicIntRange(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		anyIntRange := &IntRange{}
		intRange1_2 := NewIncludedEndIntRange(INT_1, INT_2)
		intRange1_2UnsupportedStep := NewIncludedEndIntRange(INT_1, INT_2)
		intRange1_2UnsupportedStep.isStepNotOne = true
		intRangeExclusiveEnd1_2 := NewExcludedEndIntRange(INT_1, INT_2)

		assertTest(t, anyIntRange, anyIntRange)
		assertTest(t, anyIntRange, &IntRange{})
		assertTest(t, anyIntRange, intRange1_2)
		assertTest(t, anyIntRange, intRange1_2UnsupportedStep)
		assertTest(t, anyIntRange, intRangeExclusiveEnd1_2)
		assertTestFalse(t, anyIntRange, ANY_STRING)
		assertTestFalse(t, anyIntRange, ANY_INT)

		//check intRange1_2
		assertTest(t, intRange1_2, intRange1_2)
		assertTestFalse(t, intRange1_2, anyIntRange)
		assertTestFalse(t, intRange1_2, intRangeExclusiveEnd1_2)
		assertTestFalse(t, intRange1_2, intRange1_2UnsupportedStep)
		assertTestFalse(t, intRange1_2, ANY_INT)

		//check intRange1_2UnsupportedStep
		assertTest(t, intRange1_2UnsupportedStep, intRange1_2UnsupportedStep)
		assertTestFalse(t, intRange1_2UnsupportedStep, anyIntRange)
		assertTestFalse(t, intRange1_2UnsupportedStep, intRangeExclusiveEnd1_2)
		assertTestFalse(t, intRange1_2UnsupportedStep, intRange1_2)
		assertTestFalse(t, intRange1_2UnsupportedStep, ANY_INT)

		//check intRangeExclusiveEnd1_2
		assertTest(t, intRangeExclusiveEnd1_2, intRangeExclusiveEnd1_2)
		assertTestFalse(t, intRangeExclusiveEnd1_2, anyIntRange)
		assertTestFalse(t, intRangeExclusiveEnd1_2, intRange1_2)
		assertTestFalse(t, intRangeExclusiveEnd1_2, intRange1_2UnsupportedStep)
		assertTestFalse(t, intRangeExclusiveEnd1_2, ANY_INT)
	})

	t.Run("Contains()", func(t *testing.T) {
		anyIntRange := &IntRange{}
		assertMayContainButNotCertain(t, anyIntRange, INT_0)
		assertMayContainButNotCertain(t, anyIntRange, INT_1)

		intRange1_2 := NewIncludedEndIntRange(INT_1, INT_2)
		assertContains(t, intRange1_2, INT_1)
		assertContains(t, intRange1_2, INT_2)
		assertCannotPossiblyContain(t, intRange1_2, INT_0)
		assertCannotPossiblyContain(t, intRange1_2, INT_3)

		intRange1_2ExcludedEnd := NewExcludedEndIntRange(INT_1, INT_2)
		assertContains(t, intRange1_2ExcludedEnd, INT_1)
		assertCannotPossiblyContain(t, intRange1_2ExcludedEnd, INT_0)
		assertCannotPossiblyContain(t, intRange1_2ExcludedEnd, INT_2)
		assertCannotPossiblyContain(t, intRange1_2ExcludedEnd, INT_3)

		intRangeUnsupportedStep := NewIncludedEndIntRange(INT_1, INT_2)
		intRangeUnsupportedStep.isStepNotOne = true
		assertMayContainButNotCertain(t, intRangeUnsupportedStep, INT_1)
		assertMayContainButNotCertain(t, intRangeUnsupportedStep, INT_2)
		assertCannotPossiblyContain(t, intRange1_2, INT_0)
		assertCannotPossiblyContain(t, intRange1_2, INT_3)
	})

}

func TestSymbolicFloatRange(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		anyFloatRange := &FloatRange{}
		floatRange1_2 := NewIncludedEndFloatRange(FLOAT_1, FLOAT_2)
		floatRangeExclusiveEnd1_2 := NewExcludedEndFloatRange(FLOAT_1, FLOAT_2)

		assertTest(t, anyFloatRange, anyFloatRange)
		assertTest(t, anyFloatRange, &FloatRange{})
		assertTest(t, anyFloatRange, floatRange1_2)
		assertTest(t, anyFloatRange, floatRangeExclusiveEnd1_2)
		assertTestFalse(t, anyFloatRange, ANY_STRING)
		assertTestFalse(t, anyFloatRange, ANY_FLOAT)

		//check floatRange1_2
		assertTest(t, floatRange1_2, floatRange1_2)
		assertTestFalse(t, floatRange1_2, anyFloatRange)
		assertTestFalse(t, floatRange1_2, floatRangeExclusiveEnd1_2)
		assertTestFalse(t, floatRange1_2, ANY_FLOAT)

		//check floatRangeExclusiveEnd1_2
		assertTest(t, floatRangeExclusiveEnd1_2, floatRangeExclusiveEnd1_2)
		assertTestFalse(t, floatRangeExclusiveEnd1_2, anyFloatRange)
		assertTestFalse(t, floatRangeExclusiveEnd1_2, floatRange1_2)
		assertTestFalse(t, floatRangeExclusiveEnd1_2, ANY_FLOAT)
	})

	t.Run("Contains()", func(t *testing.T) {
		anyFloatRange := &FloatRange{}
		assertMayContainButNotCertain(t, anyFloatRange, FLOAT_0)
		assertMayContainButNotCertain(t, anyFloatRange, FLOAT_1)

		floatRange1_2 := NewIncludedEndFloatRange(FLOAT_1, FLOAT_2)
		assertContains(t, floatRange1_2, FLOAT_1)
		assertContains(t, floatRange1_2, FLOAT_2)
		assertCannotPossiblyContain(t, floatRange1_2, FLOAT_0)
		assertCannotPossiblyContain(t, floatRange1_2, FLOAT_3)

		intRange1_2ExcludedEnd := NewExcludedEndFloatRange(FLOAT_1, FLOAT_2)
		assertContains(t, intRange1_2ExcludedEnd, FLOAT_1)
		assertCannotPossiblyContain(t, intRange1_2ExcludedEnd, FLOAT_0)
		assertCannotPossiblyContain(t, intRange1_2ExcludedEnd, FLOAT_2)
		assertCannotPossiblyContain(t, intRange1_2ExcludedEnd, FLOAT_3)
	})
}

func TestSymbolicRuneRange(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		runeRange := &RuneRange{}

		assertTest(t, runeRange, runeRange)
		assertTest(t, runeRange, &RuneRange{})
		assertTestFalse(t, runeRange, &String{})
		assertTestFalse(t, runeRange, &Int{})
	})

}

func TestSymbolicAnyIterable(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		anyIterable := &AnyIterable{}

		assertTest(t, anyIterable, anyIterable)
		assertTest(t, anyIterable, NewList())
		assertTest(t, anyIterable, NewListOf(ANY_SERIALIZABLE))
		assertTest(t, anyIterable, NewListOf(&Int{}))
		assertTestFalse(t, anyIterable, &Int{})
	})

}

func symbolicGoFn(ctx *Context, list *List, args ...Value) *List {
	return list
}
