package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRuneSlice(t *testing.T) {

	t.Run("insertSequence", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		slice := NewRuneSlice([]rune("ab"))
		slice.insertSequence(ctx, NewRuneSlice([]rune("xy")), 1)

		assert.Equal(t, []rune("axyb"), slice.elements)
	})

	t.Run("removePositionRange", func(t *testing.T) {

		t.Run("trailing sub slice", func(t *testing.T) {
			ctx := NewContext(ContextConfig{})
			slice := NewRuneSlice([]rune("abc"))
			slice.removePositionRange(ctx, NewIncludedEndIntRange(1, 2))

			assert.Equal(t, []rune("a"), slice.elements)
		})

		t.Run("leading sub slice", func(t *testing.T) {
			ctx := NewContext(ContextConfig{})
			slice := NewRuneSlice([]rune("abc"))
			slice.removePositionRange(ctx, NewIncludedEndIntRange(0, 1))

			assert.Equal(t, []rune("c"), slice.elements)
		})

		t.Run("sub slice in the middle", func(t *testing.T) {
			ctx := NewContext(ContextConfig{})
			slice := NewRuneSlice([]rune("abcd"))
			slice.removePositionRange(ctx, NewIncludedEndIntRange(1, 2))

			assert.Equal(t, []rune("ad"), slice.elements)
		})
	})
}

func TestStringConcatenation(t *testing.T) {
	ctx := NewContexWithEmptyState(ContextConfig{}, nil)

	t.Run("At", func(t *testing.T) {
		concatenation := &StringConcatenation{
			elements: []StringLike{String("a"), String("b")},
			totalLen: 2,
		}

		assert.Equal(t, Byte('a'), concatenation.At(ctx, 0))
		assert.Equal(t, Byte('b'), concatenation.At(ctx, 1))

		concatenation = &StringConcatenation{
			elements: []StringLike{String("ab"), String("c")},
			totalLen: 2,
		}

		assert.Equal(t, Byte('a'), concatenation.At(ctx, 0))
		assert.Equal(t, Byte('b'), concatenation.At(ctx, 1))
		assert.Equal(t, Byte('c'), concatenation.At(ctx, 2))

		concatenation = &StringConcatenation{
			elements: []StringLike{String("ab"), String("cd")},
			totalLen: 2,
		}

		assert.Equal(t, Byte('a'), concatenation.At(ctx, 0))
		assert.Equal(t, Byte('b'), concatenation.At(ctx, 1))
		assert.Equal(t, Byte('c'), concatenation.At(ctx, 2))
		assert.Equal(t, Byte('d'), concatenation.At(ctx, 3))
	})

}
