package symbolic

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
