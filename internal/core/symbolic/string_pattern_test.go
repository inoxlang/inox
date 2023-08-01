package symbolic

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSymbolicExactStringValuePattern(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		anyStr := ANY_EXACT_STR_PATTERN

		assert.True(t, anyStr.Test(ANY_EXACT_STR_PATTERN))
		assert.True(t, anyStr.Test(NewExactStringPattern(NewString(""))))
		assert.True(t, anyStr.Test(NewExactStringPattern(NewString("1"))))
		assert.False(t, anyStr.Test(ANY_INT))
		assert.False(t, anyStr.Test(ANY_PATTERN))

		emptyStr := NewExactStringPattern(NewString(""))

		assert.True(t, emptyStr.Test(NewExactStringPattern(NewString(""))))
		assert.False(t, emptyStr.Test(NewExactStringPattern(NewString("1"))))
		assert.False(t, emptyStr.Test(ANY_EXACT_STR_PATTERN))
		assert.False(t, emptyStr.Test(ANY_INT))
		assert.False(t, emptyStr.Test(ANY_PATTERN))
	})

	t.Run("TestValue()", func(t *testing.T) {
		anyStr := ANY_EXACT_STR_PATTERN

		assert.False(t, anyStr.TestValue(NewString("")))
		assert.False(t, anyStr.TestValue(NewString("1")))
		assert.False(t, anyStr.TestValue(ANY_SERIALIZABLE))
		assert.False(t, anyStr.TestValue(anyStr))

		emptyStr := NewExactStringPattern(NewString(""))

		assert.True(t, emptyStr.TestValue(NewString("")))
		assert.False(t, emptyStr.TestValue(NewString("1")))
		assert.False(t, emptyStr.TestValue(ANY_SERIALIZABLE))
		assert.False(t, emptyStr.TestValue(emptyStr))
	})

}
