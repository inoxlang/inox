package symbolic

import (
	"context"
	"testing"

	"github.com/inoxlang/inox/internal/ast"
	"github.com/stretchr/testify/assert"
)

func TestSymbolicString(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		anyStr := &String{}

		assertTest(t, anyStr, anyStr)
		assertTest(t, anyStr, &String{})
		assertTest(t, anyStr, NewString("x"))
		assertTest(t, anyStr, NewStringWithLengthRange(1, 3))
		assertTestFalse(t, anyStr, &Int{})

		strX := NewString("x")

		assertTest(t, strX, strX)
		assertTest(t, strX, NewString("x"))
		assertTestFalse(t, strX, NewString("xx"))
		assertTestFalse(t, strX, &String{})
		assertTestFalse(t, strX, NewStringWithLengthRange(0, 3))
		assertTestFalse(t, strX, &Int{})

		strMinLen1 := NewStringWithLengthRange(1, 1_000_000)

		assertTest(t, strMinLen1, strMinLen1)
		assertTest(t, strMinLen1, NewStringWithLengthRange(1, 1_000_000))
		assertTest(t, strMinLen1, NewStringWithLengthRange(2, 1_000_000))
		assertTest(t, strMinLen1, NewStringWithLengthRange(2, 3))
		assertTest(t, strMinLen1, NewStringMatchingPattern(NewLengthCheckingStringPattern(1, 2)))
		assertTest(t, strMinLen1, NewStringMatchingPattern(NewLengthCheckingStringPattern(1, 3)))
		assertTest(t, strMinLen1, NewString("x"))
		assertTest(t, strMinLen1, NewString("xx"))
		assertTestFalse(t, strMinLen1, EMPTY_STRING)
		assertTestFalse(t, strMinLen1, &String{})
		assertTestFalse(t, strMinLen1, NewStringMatchingPattern(ANY_SEQ_STRING_PATTERN))
		assertTestFalse(t, strMinLen1, NewStringMatchingPattern(NewLengthCheckingStringPattern(0, 1)))
		assertTestFalse(t, strMinLen1, NewStringMatchingPattern(NewLengthCheckingStringPattern(0, 2)))
		assertTestFalse(t, strMinLen1, NewStringWithLengthRange(0, 1))
		assertTestFalse(t, strMinLen1, NewStringWithLengthRange(0, 2))
		assertTestFalse(t, strMinLen1, &Int{})

		strMaxLen10 := NewStringWithLengthRange(0, 10)

		assertTest(t, strMaxLen10, strMaxLen10)
		assertTest(t, strMaxLen10, NewStringWithLengthRange(0, 1))
		assertTest(t, strMaxLen10, NewStringWithLengthRange(0, 2))
		assertTest(t, strMaxLen10, NewStringWithLengthRange(2, 3))
		assertTest(t, strMaxLen10, NewStringWithLengthRange(9, 10))
		assertTest(t, strMaxLen10, NewStringMatchingPattern(NewLengthCheckingStringPattern(0, 1)))
		assertTest(t, strMaxLen10, NewStringMatchingPattern(NewLengthCheckingStringPattern(1, 3)))
		assertTest(t, strMaxLen10, NewString("x"))
		assertTest(t, strMaxLen10, NewString("xx"))
		assertTest(t, strMaxLen10, EMPTY_STRING)
		assertTestFalse(t, strMaxLen10, &String{})
		assertTestFalse(t, strMaxLen10, NewStringMatchingPattern(ANY_SEQ_STRING_PATTERN))
		assertTestFalse(t, strMaxLen10, NewStringMatchingPattern(NewLengthCheckingStringPattern(11, 12)))
		assertTestFalse(t, strMaxLen10, NewStringMatchingPattern(NewLengthCheckingStringPattern(0, 11)))
		assertTestFalse(t, strMaxLen10, NewStringWithLengthRange(10, 11))
		assertTestFalse(t, strMaxLen10, NewStringWithLengthRange(1, 11))
		assertTestFalse(t, strMaxLen10, NewStringWithLengthRange(2, 11))
		assertTestFalse(t, strMaxLen10, &Int{})

		strMatchingSeq1 := NewStringMatchingPattern(ANY_SEQ_STRING_PATTERN)

		assertTest(t, strMatchingSeq1, strMatchingSeq1)
		assertTestFalse(t, strMatchingSeq1, NewStringMatchingPattern(NewExactStringPatternWithConcreteValue(NewString("x"))))
		assertTestFalse(t, strMatchingSeq1, NewString("x"))
		assertTestFalse(t, strMatchingSeq1, NewString("xx"))
		assertTestFalse(t, strMatchingSeq1, NewStringWithLengthRange(1, 3))
		assertTestFalse(t, strMatchingSeq1, &String{})
		assertTestFalse(t, strMatchingSeq1, &Int{})

		strMatchingSeq2 := NewStringMatchingPattern(NewSequenceStringPattern(&ast.ComplexStringPatternPiece{}, &ast.Chunk{}))

		assertTest(t, strMatchingSeq2, strMatchingSeq2)
		assertTest(t, strMatchingSeq2, NewStringMatchingPattern(strMatchingSeq2.pattern))
		assertTestFalse(t, strMatchingSeq2, strMatchingSeq1)
		assertTestFalse(t, strMatchingSeq2, NewStringMatchingPattern(NewExactStringPatternWithConcreteValue(NewString("x"))))
		assertTestFalse(t, strMatchingSeq2, NewStringMatchingPattern(NewExactStringPatternWithConcreteValue(NewString("x"))))
		assertTestFalse(t, strMatchingSeq2, NewString("x"))
		assertTestFalse(t, strMatchingSeq2, NewString("xx"))
		assertTestFalse(t, strMatchingSeq2, NewStringWithLengthRange(1, 3))
		assertTestFalse(t, strMatchingSeq2, &String{})
		assertTestFalse(t, strMatchingSeq2, &Int{})
	})
}

func TestSymbolicCheckedString(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		str := &CheckedString{}

		assertTest(t, str, str)
		assertTest(t, str, &CheckedString{})
		assertTestFalse(t, str, &String{})
		assertTestFalse(t, str, &Int{})
	})

}

func TesyAnyStringLike(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		strLike := &AnyStringLike{}

		assertTest(t, strLike, strLike)
		assertTest(t, strLike, &String{})
		assertTest(t, strLike, &StringConcatenation{})
		assertTestFalse(t, strLike, &Int{})
	})

}

func TestSymbolicStringConcatenation(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		concat := &StringConcatenation{}

		assertTest(t, concat, concat)
		assertTest(t, concat, &StringConcatenation{})
		assertTestFalse(t, concat, &String{})
		assertTestFalse(t, concat, &Int{})
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

			state.consumeSymbolicGoFunctionErrors(func(msg string, optionalLocation ast.Node) {
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
			state.consumeSymbolicGoFunctionErrors(func(msg string, optionalLocation ast.Node) {
				assert.Fail(t, "no error expected")
			})
		})

		t.Run("adding non-rune value", func(t *testing.T) {
			ctx := NewSymbolicContext(dummyConcreteContext{context.Background()}, nil, nil)
			state := newSymbolicState(ctx, nil)

			slice := NewRuneSlice()
			slice.insertSequence(ctx, NewList(ANY_STRING), NewInt(0))

			updatedSelf, ok := state.consumeUpdatedSelf()
			assert.False(t, ok)
			assert.Nil(t, updatedSelf)

			called := false

			state.consumeSymbolicGoFunctionErrors(func(msg string, optionalLocation ast.Node) {
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

			state.consumeSymbolicGoFunctionErrors(func(msg string, optionalLocation ast.Node) {
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
			state.consumeSymbolicGoFunctionErrors(func(msg string, optionalLocation ast.Node) {
				assert.Fail(t, "no error expected")
			})
		})

		t.Run("adding non-byte value", func(t *testing.T) {
			ctx := NewSymbolicContext(dummyConcreteContext{context.Background()}, nil, nil)
			state := newSymbolicState(ctx, nil)

			slice := NewRuneSlice()
			slice.appendSequence(ctx, NewList(ANY_STRING))

			updatedSelf, ok := state.consumeUpdatedSelf()
			assert.False(t, ok)
			assert.Nil(t, updatedSelf)

			called := false

			state.consumeSymbolicGoFunctionErrors(func(msg string, optionalLocation ast.Node) {
				called = true
				assert.Equal(t, fmtHasElementsOfType(slice, ANY_RUNE), msg)
			})
			assert.True(t, called)
		})
	})
}
