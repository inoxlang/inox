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
