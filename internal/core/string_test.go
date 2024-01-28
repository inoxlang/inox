package core

import (
	"strings"
	"testing"

	"github.com/inoxlang/inox/internal/utils"
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
			slice.removePositionRange(ctx, NewIntRange(1, 2))

			assert.Equal(t, []rune("a"), slice.elements)
		})

		t.Run("leading sub slice", func(t *testing.T) {
			ctx := NewContext(ContextConfig{})
			slice := NewRuneSlice([]rune("abc"))
			slice.removePositionRange(ctx, NewIntRange(0, 1))

			assert.Equal(t, []rune("c"), slice.elements)
		})

		t.Run("sub slice in the middle", func(t *testing.T) {
			ctx := NewContext(ContextConfig{})
			slice := NewRuneSlice([]rune("abcd"))
			slice.removePositionRange(ctx, NewIntRange(1, 2))

			assert.Equal(t, []rune("ad"), slice.elements)
		})
	})
}

func TestStringConcatenation(t *testing.T) {
	ctx := NewContexWithEmptyState(ContextConfig{}, nil)
	defer ctx.CancelGracefully()

	t.Run("At", func(t *testing.T) {
		concatenation := NewStringConcatenation(String("a"), String("b"))

		assert.Equal(t, Byte('a'), concatenation.At(ctx, 0))
		assert.Equal(t, Byte('b'), concatenation.At(ctx, 1))

		concatenation = NewStringConcatenation(String("ab"), String("c"))

		assert.Equal(t, Byte('a'), concatenation.At(ctx, 0))
		assert.Equal(t, Byte('b'), concatenation.At(ctx, 1))
		assert.Equal(t, Byte('c'), concatenation.At(ctx, 2))

		concatenation = NewStringConcatenation(String("ab"), String("cd"))

		assert.Equal(t, Byte('a'), concatenation.At(ctx, 0))
		assert.Equal(t, Byte('b'), concatenation.At(ctx, 1))
		assert.Equal(t, Byte('c'), concatenation.At(ctx, 2))
		assert.Equal(t, Byte('d'), concatenation.At(ctx, 3))
	})

	t.Run("Len & ByteLen", func(t *testing.T) {
		concatenation := NewStringConcatenation(String("a"), String("b"))

		assert.Equal(t, 2, concatenation.Len())
		assert.Equal(t, 2, concatenation.ByteLen())

		concatenation = NewStringConcatenation(String("ab"), String("c"))

		assert.Equal(t, 3, concatenation.Len())
		assert.Equal(t, 3, concatenation.ByteLen())

		concatenation = NewStringConcatenation(String("ab"), String("cd"))

		assert.Equal(t, 4, concatenation.Len())
		assert.Equal(t, 4, concatenation.ByteLen())
	})
}

func BenchmarkConcatenateStringLikes(b *testing.B) {
	longString := String(strings.Repeat("a", 1000))

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		//c := String("a") + longString   //more than 1000 bytes allocated per operation
		c := utils.Must(ConcatStringLikes(String("a"), longString))

		if c == nil {
			b.Fail()
		}
	}

}
