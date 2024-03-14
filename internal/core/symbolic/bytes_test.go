package symbolic

import (
	"context"
	"testing"

	"github.com/inoxlang/inox/internal/parse"
	"github.com/stretchr/testify/assert"
)

func TestSymbolicByteSlice(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		slice := &ByteSlice{}

		assertTest(t, slice, slice)
		assertTest(t, slice, &ByteSlice{})
		assertTestFalse(t, slice, &String{})
		assertTestFalse(t, slice, &Int{})
	})

	t.Run("insertSequence()", func(t *testing.T) {
		t.Run("adding no elements", func(t *testing.T) {
			ctx := NewSymbolicContext(dummyConcreteContext{context.Background()}, nil, nil)
			state := newSymbolicState(ctx, nil)

			slice := NewByteSlice()
			slice.insertSequence(ctx, NewList(), NewInt(0))

			updatedSelf, ok := state.consumeUpdatedSelf()
			assert.False(t, ok)
			assert.Nil(t, updatedSelf)

			state.consumeSymbolicGoFunctionErrors(func(msg string, optionalLocation parse.Node) {
				assert.Fail(t, "no error expected")
			})
		})

		t.Run("adding byte", func(t *testing.T) {
			ctx := NewSymbolicContext(dummyConcreteContext{context.Background()}, nil, nil)
			state := newSymbolicState(ctx, nil)

			slice := NewByteSlice()
			slice.insertSequence(ctx, NewList(ANY_BYTE), NewInt(0))

			updatedSelf, ok := state.consumeUpdatedSelf()
			assert.False(t, ok)
			assert.Nil(t, updatedSelf)
			state.consumeSymbolicGoFunctionErrors(func(msg string, optionalLocation parse.Node) {
				assert.Fail(t, "no error expected")
			})
		})

		t.Run("adding non-byte value", func(t *testing.T) {
			ctx := NewSymbolicContext(dummyConcreteContext{context.Background()}, nil, nil)
			state := newSymbolicState(ctx, nil)

			slice := NewByteSlice()
			slice.insertSequence(ctx, NewList(ANY_STRING), NewInt(0))

			updatedSelf, ok := state.consumeUpdatedSelf()
			assert.False(t, ok)
			assert.Nil(t, updatedSelf)

			called := false

			state.consumeSymbolicGoFunctionErrors(func(msg string, optionalLocation parse.Node) {
				called = true
				assert.Equal(t, fmtHasElementsOfType(slice, ANY_BYTE), msg)
			})
			assert.True(t, called)
		})
	})

	t.Run("appendSequence()", func(t *testing.T) {
		t.Run("adding no elements", func(t *testing.T) {
			ctx := NewSymbolicContext(dummyConcreteContext{context.Background()}, nil, nil)
			state := newSymbolicState(ctx, nil)

			slice := NewByteSlice()
			slice.appendSequence(ctx, NewList())

			updatedSelf, ok := state.consumeUpdatedSelf()
			assert.False(t, ok)
			assert.Nil(t, updatedSelf)

			state.consumeSymbolicGoFunctionErrors(func(msg string, optionalLocation parse.Node) {
				assert.Fail(t, "no error expected")
			})
		})

		t.Run("adding byte", func(t *testing.T) {
			ctx := NewSymbolicContext(dummyConcreteContext{context.Background()}, nil, nil)
			state := newSymbolicState(ctx, nil)

			slice := NewByteSlice()
			slice.appendSequence(ctx, NewList(ANY_BYTE))

			updatedSelf, ok := state.consumeUpdatedSelf()
			assert.False(t, ok)
			assert.Nil(t, updatedSelf)
			state.consumeSymbolicGoFunctionErrors(func(msg string, optionalLocation parse.Node) {
				assert.Fail(t, "no error expected")
			})
		})

		t.Run("adding non-byte value", func(t *testing.T) {
			ctx := NewSymbolicContext(dummyConcreteContext{context.Background()}, nil, nil)
			state := newSymbolicState(ctx, nil)

			slice := NewByteSlice()
			slice.appendSequence(ctx, NewList(ANY_STRING))

			updatedSelf, ok := state.consumeUpdatedSelf()
			assert.False(t, ok)
			assert.Nil(t, updatedSelf)

			called := false

			state.consumeSymbolicGoFunctionErrors(func(msg string, optionalLocation parse.Node) {
				called = true
				assert.Equal(t, fmtHasElementsOfType(slice, ANY_BYTE), msg)
			})
			assert.True(t, called)
		})
	})

}

func TestSymbolicByte(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		byte := &Byte{}

		assertTest(t, byte, byte)
		assertTest(t, byte, &Byte{})
		assertTestFalse(t, byte, &Int{})
	})

}

func TestSymbolicAnyByteLike(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		bytesLike := &AnyBytesLike{}

		assertTest(t, bytesLike, bytesLike)
		assertTest(t, bytesLike, &ByteSlice{})
		assertTest(t, bytesLike, &BytesConcatenation{})
		assertTestFalse(t, bytesLike, &String{})
		assertTestFalse(t, bytesLike, &Int{})
	})

}
