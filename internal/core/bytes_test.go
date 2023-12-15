package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestByteSlice(t *testing.T) {

	t.Run("set", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		slice := NewByteSlice([]byte("ab"), true, "")
		slice.set(ctx, 0, Byte('c'))

		assert.Equal(t, []byte("cb"), slice.bytes)
	})

	t.Run("SetSlice", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		slice := NewByteSlice([]byte("ab"), true, "")
		slice.SetSlice(ctx, 0, 2, NewByteSlice([]byte("12"), true, ""))

		assert.Equal(t, []byte("12"), slice.bytes)
	})

	t.Run("insertElement", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		slice := NewByteSlice([]byte("ab"), true, "")
		slice.insertElement(ctx, Byte('c'), 0)

		assert.Equal(t, []byte("cab"), slice.bytes)
	})

	t.Run("insertSequence", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		slice := NewByteSlice([]byte("ab"), true, "")
		slice.insertSequence(ctx, NewByteSlice([]byte("xy"), true, ""), 1)

		assert.Equal(t, []byte("axyb"), slice.bytes)
	})

	t.Run("appendSequence", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		slice := NewByteSlice([]byte("ab"), true, "")
		slice.appendSequence(ctx, NewByteSlice([]byte("cd"), true, ""))

		assert.Equal(t, []byte("abcd"), slice.bytes)
	})

	t.Run("removePosition", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		slice := NewByteSlice([]byte("abc"), true, "")
		slice.removePosition(ctx, 0)

		assert.Equal(t, []byte("bc"), slice.bytes)
	})

	t.Run("removePositionRange", func(t *testing.T) {

		t.Run("trailing sub slice", func(t *testing.T) {
			ctx := NewContext(ContextConfig{})
			slice := NewByteSlice([]byte("abc"), true, "")
			slice.removePositionRange(ctx, NewIncludedEndIntRange(1, 2))

			assert.Equal(t, []byte("a"), slice.bytes)
		})

		t.Run("leading sub slice", func(t *testing.T) {
			ctx := NewContext(ContextConfig{})
			slice := NewByteSlice([]byte("abc"), true, "")
			slice.removePositionRange(ctx, NewIncludedEndIntRange(0, 1))

			assert.Equal(t, []byte("c"), slice.bytes)
		})

		t.Run("sub slice in the middle", func(t *testing.T) {
			ctx := NewContext(ContextConfig{})
			slice := NewByteSlice([]byte("abcd"), true, "")
			slice.removePositionRange(ctx, NewIncludedEndIntRange(1, 2))

			assert.Equal(t, []byte("ad"), slice.bytes)
		})
	})
}
