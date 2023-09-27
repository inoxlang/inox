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

		assert.True(t, anyStr.Test(ANY_EXACT_STR_PATTERN))
		assert.True(t, anyStr.Test(NewExactStringPattern(NewString(""))))
		assert.True(t, anyStr.Test(NewExactStringPattern(NewString("1"))))
		assert.False(t, anyStr.Test(ANY_INT))
		assert.False(t, anyStr.Test(ANY_PATTERN))

		emptyStr := NewExactStringPattern(NewString(""))

		assert.True(t, emptyStr.Test(NewExactStringPattern(NewString(""))))
		assert.False(t, emptyStr.Test(NewExactStringPattern(NewString("1"))))
		assert.False(t, emptyStr.Test(ANY_EXACT_STR_PATTERN))
		assert.False(t, emptyStr.Test(ANY_INT))
		assert.False(t, emptyStr.Test(ANY_PATTERN))
	})

	t.Run("TestValue()", func(t *testing.T) {
		anyStr := ANY_EXACT_STR_PATTERN

		assert.False(t, anyStr.TestValue(NewString("")))
		assert.False(t, anyStr.TestValue(NewString("1")))
		assert.False(t, anyStr.TestValue(ANY_SERIALIZABLE))
		assert.False(t, anyStr.TestValue(anyStr))

		emptyStr := NewExactStringPattern(NewString(""))

		assert.True(t, emptyStr.TestValue(NewString("")))
		assert.False(t, emptyStr.TestValue(NewString("1")))
		assert.False(t, emptyStr.TestValue(ANY_SERIALIZABLE))
		assert.False(t, emptyStr.TestValue(emptyStr))
	})

}

func TestLengthCheckingStringPattern(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		any := ANY_LENGTH_CHECKING_STRING_PATTERN

		assert.True(t, any.Test(any))
		assert.True(t, any.Test(NewLengthCheckingStringPattern(0, 2)))
		assert.True(t, any.Test(NewLengthCheckingStringPattern(1, 2)))
		assert.True(t, any.Test(NewLengthCheckingStringPattern(0, math.MaxInt64)))
		assert.True(t, any.Test(NewLengthCheckingStringPattern(1, math.MaxInt64)))
		assert.False(t, any.Test(ANY_STR_PATTERN))
		assert.False(t, any.Test(ANY_INT))
		assert.False(t, any.Test(ANY_PATTERN))

		maxLen1 := NewLengthCheckingStringPattern(0, 1)

		assert.True(t, maxLen1.Test(maxLen1))
		assert.False(t, maxLen1.Test(NewLengthCheckingStringPattern(0, 2)))
		assert.False(t, maxLen1.Test(NewLengthCheckingStringPattern(1, 2)))
		assert.False(t, maxLen1.Test(NewLengthCheckingStringPattern(1, 3)))
		assert.False(t, maxLen1.Test(ANY_STR_PATTERN))
		assert.False(t, maxLen1.Test(ANY_INT))
		assert.False(t, maxLen1.Test(ANY_PATTERN))

		maxLen2 := NewLengthCheckingStringPattern(0, 2)

		assert.True(t, maxLen2.Test(maxLen2))
		assert.True(t, maxLen2.Test(NewLengthCheckingStringPattern(0, 1)))
		assert.True(t, maxLen2.Test(NewLengthCheckingStringPattern(1, 2)))
		assert.False(t, maxLen2.Test(ANY_STR_PATTERN))
		assert.False(t, maxLen2.Test(NewLengthCheckingStringPattern(2, 3)))
		assert.False(t, maxLen2.Test(NewLengthCheckingStringPattern(1, 3)))
		assert.False(t, maxLen2.Test(ANY_INT))
		assert.False(t, maxLen2.Test(ANY_PATTERN))

		minLen1MaxLen3 := NewLengthCheckingStringPattern(1, 3)

		assert.True(t, minLen1MaxLen3.Test(minLen1MaxLen3))
		assert.True(t, minLen1MaxLen3.Test(NewLengthCheckingStringPattern(1, 2)))
		assert.True(t, minLen1MaxLen3.Test(NewLengthCheckingStringPattern(1, 3)))
		assert.True(t, minLen1MaxLen3.Test(NewLengthCheckingStringPattern(2, 3)))
		assert.False(t, minLen1MaxLen3.Test(ANY_STR_PATTERN))
		assert.False(t, minLen1MaxLen3.Test(NewLengthCheckingStringPattern(0, 3)))
		assert.False(t, minLen1MaxLen3.Test(NewLengthCheckingStringPattern(1, 4)))
		assert.False(t, minLen1MaxLen3.Test(ANY_INT))
		assert.False(t, minLen1MaxLen3.Test(ANY_PATTERN))
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
				assert.Equal(t, testCase.ok, testCase.pattern.TestValue(testCase.value))

				val := testCase.pattern.SymbolicValue()
				assert.Equal(t, testCase.ok, val.Test(testCase.value))
			})
		}
	})

}

func TestSequenceStringPattern(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		anySeqStr := ANY_SEQ_STRING_PATTERN

		assert.True(t, anySeqStr.Test(ANY_SEQ_STRING_PATTERN))
		assert.False(t, anySeqStr.Test(ANY_STR_PATTERN))
		assert.False(t, anySeqStr.Test(ANY_INT))
		assert.False(t, anySeqStr.Test(ANY_PATTERN))
	})

	t.Run("TestValue()", func(t *testing.T) {
		anySeqStr := ANY_SEQ_STRING_PATTERN

		assert.False(t, anySeqStr.TestValue(NewString("")))
		assert.False(t, anySeqStr.TestValue(NewString("1")))
		assert.False(t, anySeqStr.TestValue(ANY_SERIALIZABLE))
		assert.False(t, anySeqStr.TestValue(anySeqStr))
	})

}
