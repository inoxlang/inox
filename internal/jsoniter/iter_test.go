package jsoniter

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReadObjectMinimizeAllocationsCB(t *testing.T) {

	t.Run("empty", func(t *testing.T) {

		it := NewIterator(ConfigDefault).ResetBytes([]byte("{}"))
		it.ReadObjectMinimizeAllocationsCB(func(it *Iterator, key []byte, allocated bool) bool {
			assert.FailNow(t, "callback function should not be called")
			return true
		})

		assert.NoError(t, it.Error)
	})

	t.Run("single property name (ASCII)", func(t *testing.T) {
		it := NewIterator(ConfigDefault).ResetBytes([]byte(`{"a":1}`))
		it.ReadObjectMinimizeAllocationsCB(func(it *Iterator, key []byte, allocated bool) bool {
			assert.False(t, allocated)
			assert.Equal(t, "a", string(key))

			assert.Equal(t, 1, it.ReadInt())
			return true
		})

		assert.NoError(t, it.Error)
	})

	t.Run("single property name (ASCII): empty array value", func(t *testing.T) {
		it := NewIterator(ConfigDefault).ResetBytes([]byte(`{"a":[]}`))
		it.ReadObjectMinimizeAllocationsCB(func(it *Iterator, key []byte, allocated bool) bool {
			assert.False(t, allocated)
			assert.Equal(t, "a", string(key))

			it.ReadArrayCB(func(i *Iterator) bool {
				assert.FailNow(t, "callback function should not be called")
				return true
			})

			return true
		})

		assert.NoError(t, it.Error)
	})

	t.Run("single property name (ASCII): string value", func(t *testing.T) {
		it := NewIterator(ConfigDefault).ResetBytes([]byte(`{"a":"b"}`))
		it.ReadObjectMinimizeAllocationsCB(func(it *Iterator, key []byte, allocated bool) bool {
			assert.False(t, allocated)
			assert.Equal(t, "a", string(key))
			assert.Equal(t, "b", it.ReadString())
			return true
		})

		assert.NoError(t, it.Error)
	})

	t.Run("two property names (ASCII)", func(t *testing.T) {
		it := NewIterator(ConfigDefault).ResetBytes([]byte(`{"a":1,"b":2}`))

		callCount := 0
		it.ReadObjectMinimizeAllocationsCB(func(it *Iterator, key []byte, allocated bool) bool {
			assert.False(t, allocated)
			if callCount == 0 {
				assert.Equal(t, "a", string(key))
				assert.Equal(t, 1, it.ReadInt())
			} else {
				assert.Equal(t, "b", string(key))
				assert.Equal(t, 2, it.ReadInt())
			}

			callCount++
			return true
		})

		assert.NoError(t, it.Error)
		assert.Equal(t, 2, callCount)
	})

	t.Run("three property names (ASCII)", func(t *testing.T) {
		it := NewIterator(ConfigDefault).ResetBytes([]byte(`{"a":1,"b":2,"c":3}`))

		callCount := 0
		it.ReadObjectMinimizeAllocationsCB(func(it *Iterator, key []byte, allocated bool) bool {
			assert.False(t, allocated)

			switch callCount {
			case 0:
				assert.Equal(t, "a", string(key))
				assert.Equal(t, 1, it.ReadInt())
			case 1:
				assert.Equal(t, "b", string(key))
				assert.Equal(t, 2, it.ReadInt())
			case 2:
				assert.Equal(t, "c", string(key))
				assert.Equal(t, 3, it.ReadInt())
			}

			callCount++
			return true
		})

		assert.NoError(t, it.Error)
		assert.Equal(t, 3, callCount)
	})
}
