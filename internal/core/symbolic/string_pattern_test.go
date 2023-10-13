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
		assertTest(t, anyStr, NewExactStringPattern(NewString("")))
		assertTest(t, anyStr, NewExactStringPattern(NewString("1")))
		assertTestFalse(t, anyStr, ANY_INT)
		assertTestFalse(t, anyStr, ANY_PATTERN)

		emptyStr := NewExactStringPattern(NewString(""))

		assertTest(t, emptyStr, NewExactStringPattern(NewString("")))
		assertTestFalse(t, emptyStr, NewExactStringPattern(NewString("1")))
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

		emptyStr := NewExactStringPattern(NewString(""))

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
			value   SymbolicValue
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
