package internal

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSymbolicString(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		str := &String{}

		assert.True(t, str.Test(str))
		assert.True(t, str.Test(&String{}))
		assert.False(t, str.Test(&Int{}))
	})

	t.Run("IsWidenable()", func(t *testing.T) {
		assert.False(t, (&String{}).IsWidenable())
	})

	t.Run("Widen()", func(t *testing.T) {
		str := &String{}

		assert.False(t, str.IsWidenable())

		widened, ok := str.Widen()
		assert.False(t, ok)
		assert.Nil(t, widened)
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

	t.Run("IsWidenable()", func(t *testing.T) {
		assert.False(t, (&CheckedString{}).IsWidenable())
	})

	t.Run("Widen()", func(t *testing.T) {
		str := &CheckedString{}

		assert.False(t, str.IsWidenable())

		widened, ok := str.Widen()
		assert.False(t, ok)
		assert.Nil(t, widened)
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

	t.Run("IsWidenable()", func(t *testing.T) {
		assert.False(t, (&AnyStringLike{}).IsWidenable())
	})

	t.Run("Widen()", func(t *testing.T) {
		strLike := &AnyStringLike{}

		assert.False(t, strLike.IsWidenable())

		widened, ok := strLike.Widen()
		assert.False(t, ok)
		assert.Nil(t, widened)
	})
}
