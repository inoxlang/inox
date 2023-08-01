package symbolic

import (
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
