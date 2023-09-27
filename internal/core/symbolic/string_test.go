package symbolic

import (
	"context"
	"testing"

	"github.com/inoxlang/inox/internal/parse"
	"github.com/stretchr/testify/assert"
)

func TestSymbolicString(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		anyStr := &String{}

		assert.True(t, anyStr.Test(anyStr))
		assert.True(t, anyStr.Test(&String{}))
		assert.True(t, anyStr.Test(NewString("x")))
		assert.True(t, anyStr.Test(NewStringWithLengthRange(1, 3)))
		assert.False(t, anyStr.Test(&Int{}))

		strX := NewString("x")

		assert.True(t, strX.Test(strX))
		assert.True(t, strX.Test(NewString("x")))
		assert.False(t, strX.Test(NewString("xx")))
		assert.False(t, strX.Test(&String{}))
		assert.False(t, strX.Test(NewStringWithLengthRange(0, 3)))
		assert.False(t, strX.Test(&Int{}))

		strMinLen1 := NewStringWithLengthRange(1, 1_000_000)

		assert.True(t, strMinLen1.Test(strMinLen1))
		assert.True(t, strMinLen1.Test(NewStringWithLengthRange(1, 1_000_000)))
		assert.True(t, strMinLen1.Test(NewStringWithLengthRange(2, 1_000_000)))
		assert.True(t, strMinLen1.Test(NewStringWithLengthRange(2, 3)))
		assert.True(t, strMinLen1.Test(NewStringMatchingPattern(NewLengthCheckingStringPattern(1, 2))))
		assert.True(t, strMinLen1.Test(NewStringMatchingPattern(NewLengthCheckingStringPattern(1, 3))))
		assert.True(t, strMinLen1.Test(NewString("x")))
		assert.True(t, strMinLen1.Test(NewString("xx")))
		assert.False(t, strMinLen1.Test(EMPTY_STRING))
		assert.False(t, strMinLen1.Test(&String{}))
		assert.False(t, strMinLen1.Test(NewStringMatchingPattern(ANY_SEQ_STRING_PATTERN)))
		assert.False(t, strMinLen1.Test(NewStringMatchingPattern(NewLengthCheckingStringPattern(0, 1))))
		assert.False(t, strMinLen1.Test(NewStringMatchingPattern(NewLengthCheckingStringPattern(0, 2))))
		assert.False(t, strMinLen1.Test(NewStringWithLengthRange(0, 1)))
		assert.False(t, strMinLen1.Test(NewStringWithLengthRange(0, 2)))
		assert.False(t, strMinLen1.Test(&Int{}))

		strMaxLen10 := NewStringWithLengthRange(0, 10)

		assert.True(t, strMaxLen10.Test(strMaxLen10))
		assert.True(t, strMaxLen10.Test(NewStringWithLengthRange(0, 1)))
		assert.True(t, strMaxLen10.Test(NewStringWithLengthRange(0, 2)))
		assert.True(t, strMaxLen10.Test(NewStringWithLengthRange(2, 3)))
		assert.True(t, strMaxLen10.Test(NewStringWithLengthRange(9, 10)))
		assert.True(t, strMaxLen10.Test(NewStringMatchingPattern(NewLengthCheckingStringPattern(0, 1))))
		assert.True(t, strMaxLen10.Test(NewStringMatchingPattern(NewLengthCheckingStringPattern(1, 3))))
		assert.True(t, strMaxLen10.Test(NewString("x")))
		assert.True(t, strMaxLen10.Test(NewString("xx")))
		assert.True(t, strMaxLen10.Test(EMPTY_STRING))
		assert.False(t, strMaxLen10.Test(&String{}))
		assert.False(t, strMaxLen10.Test(NewStringMatchingPattern(ANY_SEQ_STRING_PATTERN)))
		assert.False(t, strMaxLen10.Test(NewStringMatchingPattern(NewLengthCheckingStringPattern(11, 12))))
		assert.False(t, strMaxLen10.Test(NewStringMatchingPattern(NewLengthCheckingStringPattern(0, 11))))
		assert.False(t, strMaxLen10.Test(NewStringWithLengthRange(10, 11)))
		assert.False(t, strMaxLen10.Test(NewStringWithLengthRange(1, 11)))
		assert.False(t, strMaxLen10.Test(NewStringWithLengthRange(2, 11)))
		assert.False(t, strMaxLen10.Test(&Int{}))

		strMatchingSeq1 := NewStringMatchingPattern(ANY_SEQ_STRING_PATTERN)

		assert.True(t, strMatchingSeq1.Test(strMatchingSeq1))
		assert.False(t, strMatchingSeq1.Test(NewStringMatchingPattern(NewExactStringPattern(NewString("x")))))
		assert.False(t, strMatchingSeq1.Test(NewString("x")))
		assert.False(t, strMatchingSeq1.Test(NewString("xx")))
		assert.False(t, strMatchingSeq1.Test(NewStringWithLengthRange(1, 3)))
		assert.False(t, strMatchingSeq1.Test(&String{}))
		assert.False(t, strMatchingSeq1.Test(&Int{}))

		strMatchingSeq2 := NewStringMatchingPattern(NewSequenceStringPattern(&parse.ComplexStringPatternPiece{}))

		assert.True(t, strMatchingSeq2.Test(strMatchingSeq2))
		assert.True(t, strMatchingSeq2.Test(NewStringMatchingPattern(strMatchingSeq2.pattern)))
		assert.False(t, strMatchingSeq2.Test(strMatchingSeq1))
		assert.False(t, strMatchingSeq2.Test(NewStringMatchingPattern(NewExactStringPattern(NewString("x")))))
		assert.False(t, strMatchingSeq2.Test(NewStringMatchingPattern(NewExactStringPattern(NewString("x")))))
		assert.False(t, strMatchingSeq2.Test(NewString("x")))
		assert.False(t, strMatchingSeq2.Test(NewString("xx")))
		assert.False(t, strMatchingSeq2.Test(NewStringWithLengthRange(1, 3)))
		assert.False(t, strMatchingSeq2.Test(&String{}))
		assert.False(t, strMatchingSeq2.Test(&Int{}))
	})
}

func TestSymbolicCheckedString(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		str := &CheckedString{}

		assert.True(t, str.Test(str))
		assert.True(t, str.Test(&CheckedString{}))
		assert.False(t, str.Test(&String{}))
		assert.False(t, str.Test(&Int{}))
	})

}

func TesyAnyStringLike(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		strLike := &AnyStringLike{}

		assert.True(t, strLike.Test(strLike))
		assert.True(t, strLike.Test(&String{}))
		assert.True(t, strLike.Test(&StringConcatenation{}))
		assert.False(t, strLike.Test(&Int{}))
	})

}

func TestSymbolicStringConcatenation(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		concat := &StringConcatenation{}

		assert.True(t, concat.Test(concat))
		assert.True(t, concat.Test(&StringConcatenation{}))
		assert.False(t, concat.Test(&String{}))
		assert.False(t, concat.Test(&Int{}))
	})

}

func TestRuneSlice(t *testing.T) {

	t.Run("insertSequence()", func(t *testing.T) {
		t.Run("adding no elements", func(t *testing.T) {
			ctx := NewSymbolicContext(dummyConcreteContext{context.Background()}, nil, nil)
			state := newSymbolicState(ctx, nil)

			slice := NewRuneSlice()
			slice.insertSequence(ctx, NewList(), NewInt(0))

			updatedSelf, ok := state.consumeUpdatedSelf()
			assert.False(t, ok)
			assert.Nil(t, updatedSelf)

			state.consumeSymbolicGoFunctionErrors(func(msg string) {
				assert.Fail(t, "no error expected")
			})
		})

		t.Run("adding rune", func(t *testing.T) {
			ctx := NewSymbolicContext(dummyConcreteContext{context.Background()}, nil, nil)
			state := newSymbolicState(ctx, nil)

			slice := NewRuneSlice()
			slice.insertSequence(ctx, NewList(ANY_RUNE), NewInt(0))

			updatedSelf, ok := state.consumeUpdatedSelf()
			assert.False(t, ok)
			assert.Nil(t, updatedSelf)
			state.consumeSymbolicGoFunctionErrors(func(msg string) {
				assert.Fail(t, "no error expected")
			})
		})

		t.Run("adding non-rune value", func(t *testing.T) {
			ctx := NewSymbolicContext(dummyConcreteContext{context.Background()}, nil, nil)
			state := newSymbolicState(ctx, nil)

			slice := NewRuneSlice()
			slice.insertSequence(ctx, NewList(ANY_STR), NewInt(0))

			updatedSelf, ok := state.consumeUpdatedSelf()
			assert.False(t, ok)
			assert.Nil(t, updatedSelf)

			called := false

			state.consumeSymbolicGoFunctionErrors(func(msg string) {
				called = true
				assert.Equal(t, fmtHasElementsOfType(slice, ANY_RUNE), msg)
			})
			assert.True(t, called)
		})
	})

	t.Run("appendSequence()", func(t *testing.T) {
		t.Run("adding no elements", func(t *testing.T) {
			ctx := NewSymbolicContext(dummyConcreteContext{context.Background()}, nil, nil)
			state := newSymbolicState(ctx, nil)

			slice := NewRuneSlice()
			slice.appendSequence(ctx, NewList())

			updatedSelf, ok := state.consumeUpdatedSelf()
			assert.False(t, ok)
			assert.Nil(t, updatedSelf)

			state.consumeSymbolicGoFunctionErrors(func(msg string) {
				assert.Fail(t, "no error expected")
			})
		})

		t.Run("adding rune", func(t *testing.T) {
			ctx := NewSymbolicContext(dummyConcreteContext{context.Background()}, nil, nil)
			state := newSymbolicState(ctx, nil)

			slice := NewRuneSlice()
			slice.appendSequence(ctx, NewList(ANY_RUNE))

			updatedSelf, ok := state.consumeUpdatedSelf()
			assert.False(t, ok)
			assert.Nil(t, updatedSelf)
			state.consumeSymbolicGoFunctionErrors(func(msg string) {
				assert.Fail(t, "no error expected")
			})
		})

		t.Run("adding non-byte value", func(t *testing.T) {
			ctx := NewSymbolicContext(dummyConcreteContext{context.Background()}, nil, nil)
			state := newSymbolicState(ctx, nil)

			slice := NewRuneSlice()
			slice.appendSequence(ctx, NewList(ANY_STR))

			updatedSelf, ok := state.consumeUpdatedSelf()
			assert.False(t, ok)
			assert.Nil(t, updatedSelf)

			called := false

			state.consumeSymbolicGoFunctionErrors(func(msg string) {
				called = true
				assert.Equal(t, fmtHasElementsOfType(slice, ANY_RUNE), msg)
			})
			assert.True(t, called)
		})
	})
}
