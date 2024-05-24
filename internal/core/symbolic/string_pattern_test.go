package symbolic

import (
	"fmt"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSymbolicExactStringValuePattern(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		anyStr := ANY_EXACT_STR_PATTERN

		assertTest(t, anyStr, ANY_EXACT_STR_PATTERN)
		assertTest(t, anyStr, NewExactStringPatternWithConcreteValue(NewString("")))
		assertTest(t, anyStr, NewExactStringPatternWithConcreteValue(NewString("1")))
		assertTestFalse(t, anyStr, ANY_INT)
		assertTestFalse(t, anyStr, ANY_PATTERN)

		emptyStr := NewExactStringPatternWithConcreteValue(NewString(""))

		assertTest(t, emptyStr, NewExactStringPatternWithConcreteValue(NewString("")))
		assertTestFalse(t, emptyStr, NewExactStringPatternWithConcreteValue(NewString("1")))
		assertTestFalse(t, emptyStr, ANY_EXACT_STR_PATTERN)
		assertTestFalse(t, emptyStr, ANY_INT)
		assertTestFalse(t, emptyStr, ANY_PATTERN)
	})

	t.Run("TestValue()", func(t *testing.T) {
		anyStr := ANY_EXACT_STR_PATTERN

		assertTestValueFalse(t, anyStr, NewString(""))
		assertTestValueFalse(t, anyStr, NewString("1"))
		assertTestValueFalse(t, anyStr, ANY_SERIALIZABLE)
		assertTestValueFalse(t, anyStr, anyStr)

		emptyStr := NewExactStringPatternWithConcreteValue(NewString(""))

		assertTestValue(t, emptyStr, NewString(""))
		assertTestValueFalse(t, emptyStr, NewString("1"))
		assertTestValueFalse(t, emptyStr, ANY_SERIALIZABLE)
		assertTestValueFalse(t, emptyStr, emptyStr)
	})

}

func TestLengthCheckingStringPattern(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		any := ANY_LENGTH_CHECKING_STRING_PATTERN

		assertTest(t, any, any)
		assertTest(t, any, NewLengthCheckingStringPattern(0, 2))
		assertTest(t, any, NewLengthCheckingStringPattern(1, 2))
		assertTest(t, any, NewLengthCheckingStringPattern(0, math.MaxInt64))
		assertTest(t, any, NewLengthCheckingStringPattern(1, math.MaxInt64))
		assertTestFalse(t, any, ANY_STR_PATTERN)
		assertTestFalse(t, any, ANY_INT)
		assertTestFalse(t, any, ANY_PATTERN)

		maxLen1 := NewLengthCheckingStringPattern(0, 1)

		assertTest(t, maxLen1, maxLen1)
		assertTestFalse(t, maxLen1, NewLengthCheckingStringPattern(0, 2))
		assertTestFalse(t, maxLen1, NewLengthCheckingStringPattern(1, 2))
		assertTestFalse(t, maxLen1, NewLengthCheckingStringPattern(1, 3))
		assertTestFalse(t, maxLen1, ANY_STR_PATTERN)
		assertTestFalse(t, maxLen1, ANY_INT)
		assertTestFalse(t, maxLen1, ANY_PATTERN)

		maxLen2 := NewLengthCheckingStringPattern(0, 2)

		assertTest(t, maxLen2, maxLen2)
		assertTest(t, maxLen2, NewLengthCheckingStringPattern(0, 1))
		assertTest(t, maxLen2, NewLengthCheckingStringPattern(1, 2))
		assertTestFalse(t, maxLen2, ANY_STR_PATTERN)
		assertTestFalse(t, maxLen2, NewLengthCheckingStringPattern(2, 3))
		assertTestFalse(t, maxLen2, NewLengthCheckingStringPattern(1, 3))
		assertTestFalse(t, maxLen2, ANY_INT)
		assertTestFalse(t, maxLen2, ANY_PATTERN)

		minLen1MaxLen3 := NewLengthCheckingStringPattern(1, 3)

		assertTest(t, minLen1MaxLen3, minLen1MaxLen3)
		assertTest(t, minLen1MaxLen3, NewLengthCheckingStringPattern(1, 2))
		assertTest(t, minLen1MaxLen3, NewLengthCheckingStringPattern(1, 3))
		assertTest(t, minLen1MaxLen3, NewLengthCheckingStringPattern(2, 3))
		assertTestFalse(t, minLen1MaxLen3, ANY_STR_PATTERN)
		assertTestFalse(t, minLen1MaxLen3, NewLengthCheckingStringPattern(0, 3))
		assertTestFalse(t, minLen1MaxLen3, NewLengthCheckingStringPattern(1, 4))
		assertTestFalse(t, minLen1MaxLen3, ANY_INT)
		assertTestFalse(t, minLen1MaxLen3, ANY_PATTERN)
	})

	t.Run("TestValue()", func(t *testing.T) {
		cases := []struct {
			pattern *LengthCheckingStringPattern
			value   Value
			ok      bool
		}{
			{
				ANY_LENGTH_CHECKING_STRING_PATTERN,
				NewString(""),
				true,
			},
			{
				ANY_LENGTH_CHECKING_STRING_PATTERN,
				NewString("a"),
				true,
			},
			{
				ANY_LENGTH_CHECKING_STRING_PATTERN,
				NewString("aa"),
				true,
			},
			{
				ANY_LENGTH_CHECKING_STRING_PATTERN,
				NewStringWithLengthRange(0, 1),
				true,
			},
			{
				ANY_LENGTH_CHECKING_STRING_PATTERN,
				NewStringWithLengthRange(0, 2),
				true,
			},
			{
				ANY_LENGTH_CHECKING_STRING_PATTERN,
				NewStringWithLengthRange(0, math.MaxInt64),
				true,
			},
			{
				ANY_LENGTH_CHECKING_STRING_PATTERN,
				NewStringMatchingPattern(ANY_LENGTH_CHECKING_STRING_PATTERN),
				true,
			},
			//
			{
				NewLengthCheckingStringPattern(0, 1),
				NewString(""),
				true,
			},
			{
				NewLengthCheckingStringPattern(0, 1),
				NewString("a"),
				true,
			},
			{
				NewLengthCheckingStringPattern(0, 1),
				NewStringMatchingPattern(NewLengthCheckingStringPattern(0, 1)),
				true,
			},
			{
				NewLengthCheckingStringPattern(0, 1),
				NewString("aa"),
				false,
			},
			{
				NewLengthCheckingStringPattern(0, 1),
				NewStringWithLengthRange(0, 1),
				true,
			},
			{
				NewLengthCheckingStringPattern(0, 1),
				NewStringMatchingPattern(NewLengthCheckingStringPattern(0, 2)),
				false,
			},
			{
				NewLengthCheckingStringPattern(0, 1),
				NewStringMatchingPattern(NewLengthCheckingStringPattern(1, 2)),
				false,
			},
			{
				NewLengthCheckingStringPattern(0, 1),
				NewStringWithLengthRange(0, 2),
				false,
			},
			{
				NewLengthCheckingStringPattern(0, 1),
				NewStringWithLengthRange(1, 2),
				false,
			},
			//
			{
				NewLengthCheckingStringPattern(0, 2),
				NewString("a"),
				true,
			},
			{
				NewLengthCheckingStringPattern(0, 2),
				NewString("aa"),
				true,
			},
			{
				NewLengthCheckingStringPattern(0, 2),
				NewString(""),
				true,
			},
			{
				NewLengthCheckingStringPattern(0, 2),
				NewString("aaa"),
				false,
			},
			{
				NewLengthCheckingStringPattern(0, 2),
				NewStringWithLengthRange(0, 2),
				true,
			},
			{
				NewLengthCheckingStringPattern(0, 2),
				NewStringWithLengthRange(0, 1),
				true,
			},
			{
				NewLengthCheckingStringPattern(0, 2),
				NewStringWithLengthRange(1, 2),
				true,
			},
			{
				NewLengthCheckingStringPattern(0, 2),
				NewStringWithLengthRange(0, 3),
				false,
			},
			{
				NewLengthCheckingStringPattern(0, 2),
				NewStringWithLengthRange(2, 3),
				false,
			},
			//
			{
				NewLengthCheckingStringPattern(1, 3),
				NewString("a"),
				true,
			},
			{
				NewLengthCheckingStringPattern(1, 3),
				NewString("aa"),
				true,
			},
			{
				NewLengthCheckingStringPattern(1, 3),
				NewString("aaa"),
				true,
			},
			{
				NewLengthCheckingStringPattern(1, 3),
				NewString(""),
				false,
			},
			{
				NewLengthCheckingStringPattern(1, 3),
				NewString("aaaa"),
				false,
			},
			{
				NewLengthCheckingStringPattern(1, 3),
				NewStringWithLengthRange(1, 3),
				true,
			},
			{
				NewLengthCheckingStringPattern(1, 3),
				NewStringWithLengthRange(1, 2),
				true,
			},
			{
				NewLengthCheckingStringPattern(1, 3),
				NewStringWithLengthRange(2, 3),
				true,
			},
			{
				NewLengthCheckingStringPattern(1, 3),
				NewStringWithLengthRange(0, 3),
				false,
			},
			{
				NewLengthCheckingStringPattern(1, 3),
				NewStringWithLengthRange(1, 4),
				false,
			},
			{
				NewLengthCheckingStringPattern(1, 3),
				NewStringWithLengthRange(3, 4),
				false,
			},
		}

		for _, testCase := range cases {
			s := " should match "
			if !testCase.ok {
				s = " should not match"
			}
			t.Run(t.Name()+"_"+fmt.Sprint(Stringify(testCase.pattern), s, Stringify(testCase.value)), func(t *testing.T) {
				assert.Equal(t, testCase.ok, testCase.pattern.TestValue(testCase.value, RecTestCallState{}))

				val := testCase.pattern.SymbolicValue()
				assert.Equal(t, testCase.ok, val.Test(testCase.value, RecTestCallState{}))
			})
		}
	})

}

func TestSequenceStringPattern(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		anySeqStr := ANY_SEQ_STRING_PATTERN

		assertTest(t, anySeqStr, ANY_SEQ_STRING_PATTERN)
		assertTestFalse(t, anySeqStr, ANY_STR_PATTERN)
		assertTestFalse(t, anySeqStr, ANY_INT)
		assertTestFalse(t, anySeqStr, ANY_PATTERN)
	})

	t.Run("TestValue()", func(t *testing.T) {
		anySeqStr := ANY_SEQ_STRING_PATTERN

		assertTestValueFalse(t, anySeqStr, NewString(""))
		assertTestValueFalse(t, anySeqStr, NewString("1"))
		assertTestValueFalse(t, anySeqStr, ANY_SERIALIZABLE)
		assertTestValueFalse(t, anySeqStr, anySeqStr)
	})

}

func TestIntRangeStringPattern(t *testing.T) {
	any := ANY_INT_RANGE_STRING_PATTERN
	anyIntRange := NewIntRangeStringPattern(ANY_INT_RANGE_PATTERN)
	specificIntRange1 := NewIntRangeStringPattern(NewIntRangePattern(NewIntRange(INT_1, INT_2, false)))
	specificIntRange2 := NewIntRangeStringPattern(NewIntRangePattern(NewIntRange(INT_1, INT_3, false)))

	t.Run("Test()", func(t *testing.T) {
		assertTest(t, any, any)
		assertTest(t, any, anyIntRange)
		assertTest(t, any, specificIntRange1)
		assertTest(t, any, specificIntRange2)

		assertTest(t, anyIntRange, anyIntRange)
		assertTest(t, anyIntRange, specificIntRange1)
		assertTest(t, anyIntRange, specificIntRange2)
		assertTestFalse(t, anyIntRange, any)

		assertTest(t, specificIntRange1, specificIntRange1)
		assertTestFalse(t, specificIntRange1, specificIntRange2)
		assertTestFalse(t, specificIntRange1, anyIntRange)
		assertTestFalse(t, specificIntRange1, any)
	})

	t.Run("TestValue()", func(t *testing.T) {
		//counterpart to Test() cases.

		assertTestValue(t, any, any.SymbolicValue())
		assertTestValue(t, any, anyIntRange.SymbolicValue())
		assertTestValue(t, any, specificIntRange1.SymbolicValue())
		assertTestValue(t, any, specificIntRange2.SymbolicValue())

		assertTestValue(t, anyIntRange, anyIntRange.SymbolicValue())
		assertTestValue(t, anyIntRange, specificIntRange1.SymbolicValue())
		assertTestValue(t, anyIntRange, specificIntRange2.SymbolicValue())
		assertTestValueFalse(t, anyIntRange, any.SymbolicValue())

		assertTestValue(t, specificIntRange1, specificIntRange1.SymbolicValue())
		assertTestValueFalse(t, specificIntRange1, specificIntRange2.SymbolicValue())
		assertTestValueFalse(t, specificIntRange1, anyIntRange.SymbolicValue())
		assertTestValueFalse(t, specificIntRange1, any.SymbolicValue())
	})

}

func TestFloatRangeStringPattern(t *testing.T) {
	any := ANY_FLOAT_RANGE_STRING_PATTERN
	anyFloatRange := NewFloatRangeStringPattern(ANY_FLOAT_RANGE_PATTERN)
	specificFloatRange1 := NewFloatRangeStringPattern(NewFloatRangePattern(NewIncludedEndFloatRange(FLOAT_1, FLOAT_2)))
	specificFloatRange2 := NewFloatRangeStringPattern(NewFloatRangePattern(NewIncludedEndFloatRange(FLOAT_1, FLOAT_3)))

	t.Run("Test()", func(t *testing.T) {
		assertTest(t, any, any)
		assertTest(t, any, anyFloatRange)
		assertTest(t, any, specificFloatRange1)
		assertTest(t, any, specificFloatRange2)

		assertTest(t, anyFloatRange, anyFloatRange)
		assertTest(t, anyFloatRange, specificFloatRange1)
		assertTest(t, anyFloatRange, specificFloatRange2)
		assertTestFalse(t, anyFloatRange, any)

		assertTest(t, specificFloatRange1, specificFloatRange1)
		assertTestFalse(t, specificFloatRange1, specificFloatRange2)
		assertTestFalse(t, specificFloatRange1, anyFloatRange)
		assertTestFalse(t, specificFloatRange1, any)
	})

	t.Run("TestValue()", func(t *testing.T) {
		//counterpart to Test() cases.

		assertTestValue(t, any, any.SymbolicValue())
		assertTestValue(t, any, anyFloatRange.SymbolicValue())
		assertTestValue(t, any, specificFloatRange1.SymbolicValue())
		assertTestValue(t, any, specificFloatRange2.SymbolicValue())

		assertTestValue(t, anyFloatRange, anyFloatRange.SymbolicValue())
		assertTestValue(t, anyFloatRange, specificFloatRange1.SymbolicValue())
		assertTestValue(t, anyFloatRange, specificFloatRange2.SymbolicValue())
		assertTestValueFalse(t, anyFloatRange, any.SymbolicValue())

		assertTestValue(t, specificFloatRange1, specificFloatRange1.SymbolicValue())
		assertTestValueFalse(t, specificFloatRange1, specificFloatRange2.SymbolicValue())
		assertTestValueFalse(t, specificFloatRange1, anyFloatRange.SymbolicValue())
		assertTestValueFalse(t, specificFloatRange1, any.SymbolicValue())
	})

}
