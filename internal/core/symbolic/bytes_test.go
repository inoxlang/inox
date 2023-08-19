package symbolic

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSymbolicByteSlice(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		slice := &ByteSlice{}

		assert.True(t, slice.Test(slice))
		assert.True(t, slice.Test(&ByteSlice{}))
		assert.False(t, slice.Test(&String{}))
		assert.False(t, slice.Test(&Int{}))
	})

	t.Run("insertSequence()", func(t *testing.T) {
		t.Run("adding no elements", func(t *testing.T) {
			ctx := NewSymbolicContext(testConcreteContext{context.Background()}, nil)
			state := newSymbolicState(ctx, nil)

			slice := NewByteSlice()
			slice.insertSequence(ctx, NewList(), NewInt(0))

			updatedSelf, ok := state.consumeUpdatedSelf()
			assert.False(t, ok)
			assert.Nil(t, updatedSelf)

			state.consumeSymbolicGoFunctionErrors(func(msg string) {
				assert.Fail(t, "no error expected")
			})
		})

		t.Run("adding byte", func(t *testing.T) {
			ctx := NewSymbolicContext(testConcreteContext{context.Background()}, nil)
			state := newSymbolicState(ctx, nil)

			slice := NewByteSlice()
			slice.insertSequence(ctx, NewList(ANY_BYTE), NewInt(0))

			updatedSelf, ok := state.consumeUpdatedSelf()
			assert.False(t, ok)
			assert.Nil(t, updatedSelf)
			state.consumeSymbolicGoFunctionErrors(func(msg string) {
				assert.Fail(t, "no error expected")
			})
		})

		t.Run("adding non-byte value", func(t *testing.T) {
			ctx := NewSymbolicContext(testConcreteContext{context.Background()}, nil)
			state := newSymbolicState(ctx, nil)

			slice := NewByteSlice()
			slice.insertSequence(ctx, NewList(ANY_STR), NewInt(0))

			updatedSelf, ok := state.consumeUpdatedSelf()
			assert.False(t, ok)
			assert.Nil(t, updatedSelf)

			called := false

			state.consumeSymbolicGoFunctionErrors(func(msg string) {
				called = true
				assert.Equal(t, fmtHasElementsOfType(slice, ANY_BYTE), msg)
			})
			assert.True(t, called)
		})
	})

	t.Run("appendSequence()", func(t *testing.T) {
		t.Run("adding no elements", func(t *testing.T) {
			ctx := NewSymbolicContext(testConcreteContext{context.Background()}, nil)
			state := newSymbolicState(ctx, nil)

			slice := NewByteSlice()
			slice.appendSequence(ctx, NewList())

			updatedSelf, ok := state.consumeUpdatedSelf()
			assert.False(t, ok)
			assert.Nil(t, updatedSelf)

			state.consumeSymbolicGoFunctionErrors(func(msg string) {
				assert.Fail(t, "no error expected")
			})
		})

		t.Run("adding byte", func(t *testing.T) {
			ctx := NewSymbolicContext(testConcreteContext{context.Background()}, nil)
			state := newSymbolicState(ctx, nil)

			slice := NewByteSlice()
			slice.appendSequence(ctx, NewList(ANY_BYTE))

			updatedSelf, ok := state.consumeUpdatedSelf()
			assert.False(t, ok)
			assert.Nil(t, updatedSelf)
			state.consumeSymbolicGoFunctionErrors(func(msg string) {
				assert.Fail(t, "no error expected")
			})
		})

		t.Run("adding non-byte value", func(t *testing.T) {
			ctx := NewSymbolicContext(testConcreteContext{context.Background()}, nil)
			state := newSymbolicState(ctx, nil)

			slice := NewByteSlice()
			slice.appendSequence(ctx, NewList(ANY_STR))

			updatedSelf, ok := state.consumeUpdatedSelf()
			assert.False(t, ok)
			assert.Nil(t, updatedSelf)

			called := false

			state.consumeSymbolicGoFunctionErrors(func(msg string) {
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

		assert.True(t, byte.Test(byte))
		assert.True(t, byte.Test(&Byte{}))
		assert.False(t, byte.Test(&Int{}))
	})

}

func TestSymbolicAnyByteLike(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		bytesLike := &AnyBytesLike{}

		assert.True(t, bytesLike.Test(bytesLike))
		assert.True(t, bytesLike.Test(&ByteSlice{}))
		assert.True(t, bytesLike.Test(&BytesConcatenation{}))
		assert.False(t, bytesLike.Test(&String{}))
		assert.False(t, bytesLike.Test(&Int{}))
	})

}
