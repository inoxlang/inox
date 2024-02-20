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
	ctx := NewContextWithEmptyState(ContextConfig{}, nil)
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

func TestConcatStringLikes(t *testing.T) {
	_longString := strings.Repeat("a", MAX_SMALL_STRING_SIZE_IN_LAZY_STR_CONCATENATION+1)
	longString := String(_longString)
	_shortString := "b"
	shortString := String(_shortString)
	concatenation := NewStringConcatenation(longString, longString)
	_concatenation := _longString + _longString

	{
		result, err := ConcatStringLikes(shortString, longString)
		if !assert.NoError(t, err) {
			return
		}
		assert.Equal(t, _shortString+_longString, result.GetOrBuildString())
	}

	{
		result, err := ConcatStringLikes(shortString, concatenation)
		if !assert.NoError(t, err) {
			return
		}
		assert.Equal(t, _shortString+_concatenation, result.GetOrBuildString())
	}

	{
		result, err := ConcatStringLikes(longString, shortString)
		if !assert.NoError(t, err) {
			return
		}
		assert.Equal(t, _longString+_shortString, result.GetOrBuildString())
	}

	{
		result, err := ConcatStringLikes(concatenation, shortString)
		if !assert.NoError(t, err) {
			return
		}
		assert.Equal(t, _concatenation+_shortString, result.GetOrBuildString())
	}

	{
		result, err := ConcatStringLikes(longString, shortString, longString)
		if !assert.NoError(t, err) {
			return
		}
		assert.Equal(t, _longString+_shortString+_longString, result.GetOrBuildString())
	}

	{
		result, err := ConcatStringLikes(concatenation, shortString, concatenation)
		if !assert.NoError(t, err) {
			return
		}
		assert.Equal(t, _concatenation+_shortString+_concatenation, result.GetOrBuildString())
	}

	{
		result, err := ConcatStringLikes(longString, shortString, longString, shortString)
		if !assert.NoError(t, err) {
			return
		}
		assert.Equal(t, _longString+_shortString+_longString+_shortString, result.GetOrBuildString())
	}

	{
		result, err := ConcatStringLikes(concatenation, shortString, concatenation, shortString)
		if !assert.NoError(t, err) {
			return
		}
		assert.Equal(t, _concatenation+_shortString+_concatenation+_shortString, result.GetOrBuildString())
	}
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

func TestIsSubstrof(t *testing.T) {
	ctx := NewContextWithEmptyState(ContextConfig{}, nil)
	defer ctx.CancelGracefully()

	AB_CD_CONCATENATION := NewStringConcatenation(String("ab"), String("cd"))
	ABCD_BYTES := NewMutableByteSlice([]byte("abcd"), "")
	AB_BYTES := NewMutableByteSlice([]byte("ab"), "")
	EMPTY_BYTES := NewMutableByteSlice([]byte(""), "")

	//True
	assert.True(t, isSubstrOf(ctx, String(""), String("a")))
	assert.True(t, isSubstrOf(ctx, String(""), String("abcd")))
	assert.True(t, isSubstrOf(ctx, String(""), ABCD_BYTES))
	assert.True(t, isSubstrOf(ctx, String(""), AB_CD_CONCATENATION))

	assert.True(t, isSubstrOf(ctx, EMPTY_BYTES, String("a")))
	assert.True(t, isSubstrOf(ctx, EMPTY_BYTES, String("abcd")))
	assert.True(t, isSubstrOf(ctx, EMPTY_BYTES, ABCD_BYTES))
	assert.True(t, isSubstrOf(ctx, EMPTY_BYTES, AB_CD_CONCATENATION))

	assert.True(t, isSubstrOf(ctx, String("a"), String("a")))
	assert.True(t, isSubstrOf(ctx, String("a"), String("abcd")))
	assert.True(t, isSubstrOf(ctx, String("a"), ABCD_BYTES))

	assert.True(t, isSubstrOf(ctx, AB_CD_CONCATENATION, String("abcd")))
	assert.True(t, isSubstrOf(ctx, AB_CD_CONCATENATION, String("abcd")))

	assert.True(t, isSubstrOf(ctx, AB_BYTES, String("abcd")))
	assert.True(t, isSubstrOf(ctx, AB_BYTES, AB_CD_CONCATENATION))
	assert.True(t, isSubstrOf(ctx, AB_BYTES, ABCD_BYTES))

	//False
	assert.False(t, isSubstrOf(ctx, String("aa"), String("a")))
	assert.False(t, isSubstrOf(ctx, String("aa"), String("abcd")))
	assert.False(t, isSubstrOf(ctx, String("aa"), ABCD_BYTES))
	assert.False(t, isSubstrOf(ctx, String("aa"), String("")))

	assert.False(t, isSubstrOf(ctx, AB_CD_CONCATENATION, String("a")))
	assert.False(t, isSubstrOf(ctx, AB_CD_CONCATENATION, AB_BYTES))

	assert.False(t, isSubstrOf(ctx, ABCD_BYTES, String("a")))
	assert.False(t, isSubstrOf(ctx, ABCD_BYTES, AB_BYTES))
}
