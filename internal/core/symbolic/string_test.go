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
		assert.False(t, anyStr.Test(&Int{}))

		strX := NewString("x")

		assert.True(t, strX.Test(strX))
		assert.True(t, strX.Test(NewString("x")))
		assert.False(t, strX.Test(NewString("xx")))
		assert.False(t, strX.Test(&String{}))
		assert.False(t, strX.Test(&Int{}))

		strMatchingSeq1 := NewStringMatchingPattern(ANY_SEQ_STRING_PATTERN)

		assert.True(t, strMatchingSeq1.Test(strMatchingSeq1))
		assert.False(t, strMatchingSeq1.Test(NewStringMatchingPattern(NewExactStringPattern(NewString("x")))))
		assert.False(t, strMatchingSeq1.Test(NewString("x")))
		assert.False(t, strMatchingSeq1.Test(NewString("xx")))
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
			ctx := NewSymbolicContext(testConcreteContext{context.Background()})
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
			ctx := NewSymbolicContext(testConcreteContext{context.Background()})
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
			ctx := NewSymbolicContext(testConcreteContext{context.Background()})
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
			ctx := NewSymbolicContext(testConcreteContext{context.Background()})
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
			ctx := NewSymbolicContext(testConcreteContext{context.Background()})
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
			ctx := NewSymbolicContext(testConcreteContext{context.Background()})
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
