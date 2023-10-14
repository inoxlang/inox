package in_mem_ds

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBitSet32(t *testing.T) {
	t.Run("Set", func(t *testing.T) {

		t.Run("0", func(t *testing.T) {
			bitSet := BitSet32(0)
			bitSet.Set(Bit32Index(0))

			assert.True(t, bitSet.IsSet(Bit32Index(0)))
			assert.False(t, bitSet.IsSet(Bit32Index(1)))
			assert.False(t, bitSet.IsSet(Bit32Index(2)))
		})

		t.Run("1", func(t *testing.T) {
			bitSet := BitSet32(0)
			bitSet.Set(Bit32Index(1))

			assert.True(t, bitSet.IsSet(Bit32Index(1)))
			assert.False(t, bitSet.IsSet(Bit32Index(0)))
			assert.False(t, bitSet.IsSet(Bit32Index(2)))
		})

		t.Run("31", func(t *testing.T) {
			bitSet := BitSet32(0)
			bitSet.Set(Bit32Index(31))

			assert.True(t, bitSet.IsSet(Bit32Index(31)))
			assert.False(t, bitSet.IsSet(Bit32Index(0)))
			assert.False(t, bitSet.IsSet(Bit32Index(1)))
			assert.False(t, bitSet.IsSet(Bit32Index(2)))
		})

		t.Run("32: out of bounds", func(t *testing.T) {
			bitSet := BitSet32(0)

			assert.PanicsWithError(t, ErrOutOfBoundsBit32Index.Error(), func() {
				bitSet.Set(Bit32Index(32))
			})
		})
	})

	t.Run("Unset", func(t *testing.T) {

		t.Run("0", func(t *testing.T) {
			bitSet := BitSet32(0)
			bitSet.Set(Bit32Index(0))
			bitSet.Set(Bit32Index(1))
			bitSet.Unset(Bit32Index(0))

			assert.False(t, bitSet.IsSet(Bit32Index(0)))
			assert.True(t, bitSet.IsSet(Bit32Index(1)))
			assert.False(t, bitSet.IsSet(Bit32Index(2)))
		})

		t.Run("1", func(t *testing.T) {
			bitSet := BitSet32(0)
			bitSet.Set(Bit32Index(0))
			bitSet.Set(Bit32Index(1))
			bitSet.Unset(Bit32Index(1))

			assert.True(t, bitSet.IsSet(Bit32Index(0)))
			assert.False(t, bitSet.IsSet(Bit32Index(1)))
			assert.False(t, bitSet.IsSet(Bit32Index(2)))
		})

		t.Run("31", func(t *testing.T) {
			bitSet := BitSet32(0)
			bitSet.Set(Bit32Index(31))
			bitSet.Set(Bit32Index(0))
			bitSet.Unset(Bit32Index(31))

			assert.False(t, bitSet.IsSet(Bit32Index(31)))
			assert.True(t, bitSet.IsSet(Bit32Index(0)))
			assert.False(t, bitSet.IsSet(Bit32Index(1)))
			assert.False(t, bitSet.IsSet(Bit32Index(2)))
		})

		t.Run("32: out of bounds", func(t *testing.T) {
			bitSet := BitSet32(0)

			assert.PanicsWithError(t, ErrOutOfBoundsBit32Index.Error(), func() {
				bitSet.Unset(Bit32Index(32))
			})
		})
	})
	t.Run("CountSet", func(t *testing.T) {

		t.Run("no bit set", func(t *testing.T) {
			bitSet := BitSet32(0)
			assert.Zero(t, bitSet.CountSet())
		})

		t.Run("two bits set", func(t *testing.T) {
			bitSet := BitSet32(0)

			bitSet.Set(Bit32Index(0))
			bitSet.Set(Bit32Index(2))

			assert.Equal(t, 2, bitSet.CountSet())
		})
	})

	t.Run("ForEachSet", func(t *testing.T) {
		bitSet := BitSet32(0)
		var setIndices []Bit32Index

		bitSet.Set(Bit32Index(0))
		bitSet.Set(Bit32Index(1))

		// collect the set indices
		err := bitSet.ForEachSet(func(index Bit32Index) error {
			setIndices = append(setIndices, index)
			return nil
		})

		assert.NoError(t, err)
		assert.ElementsMatch(t, []Bit32Index{0, 1}, setIndices)
	})
}
