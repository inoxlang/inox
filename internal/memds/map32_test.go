package memds

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMap32(t *testing.T) {
	t.Run("Get", func(t *testing.T) {
		// Create a new Map32 with test data
		m := &Map32[string, string]{
			size: 2,
			entries: [32]StringMap32Entry[string, string]{
				{key: "key1", value: "value1"},
				{key: "key2", value: "value2"},
			},
		}

		v, found := m.Get("key1")
		assert.True(t, found)
		assert.Equal(t, "value1", v)
	})

	t.Run("MustGet", func(t *testing.T) {
		// Create a new Map32 with test data
		m := &Map32[string, string]{
			size: 2,
			entries: [32]StringMap32Entry[string, string]{
				{key: "key1", value: "value1"},
				{key: "key2", value: "value2"},
			},
		}

		v := m.MustGet("key2")
		assert.Equal(t, "value2", v)
	})

	t.Run("Set", func(t *testing.T) {
		t.Run("empty map", func(t *testing.T) {
			// Create a new Map32 with test data
			m := &Map32[string, string]{
				size:    0,
				entries: [32]StringMap32Entry[string, string]{},
			}

			err := m.Set("key3", "value3")
			assert.NoError(t, err)
			assert.Equal(t, 1, m.Size())
			assert.False(t, m.IsFull())
		})

		t.Run("one entry: new key is greater", func(t *testing.T) {
			// Create a new Map32 with test data
			m := &Map32[string, string]{
				size: 1,
				entries: [32]StringMap32Entry[string, string]{
					{"key1", "value1"},
				},
			}

			err := m.Set("key2", "value2")
			if !assert.NoError(t, err) {
				return
			}
			assert.Equal(t, 2, m.Size())
			assert.False(t, m.IsFull())

			//test that key2 is in the map
			v, ok := m.Get("key2")
			if !assert.True(t, ok) {
				return
			}
			assert.Equal(t, "value2", v)

			//test that key1 is still in the map
			v, ok = m.Get("key1")
			if !assert.True(t, ok) {
				return
			}
			assert.Equal(t, "value1", v)
		})

		t.Run("one entry: new key is smaller", func(t *testing.T) {
			// Create a new Map32 with test data
			m := &Map32[string, string]{
				size: 1,
				entries: [32]StringMap32Entry[string, string]{
					{"key1", "value1"},
				},
			}

			err := m.Set("key0", "value0")
			if !assert.NoError(t, err) {
				return
			}
			assert.Equal(t, 2, m.Size())
			assert.False(t, m.IsFull())

			//test that key0 is in the map
			v, ok := m.Get("key0")
			if !assert.True(t, ok) {
				return
			}
			assert.Equal(t, "value0", v)

			//test that key1 is still in the map
			v, ok = m.Get("key1")
			if !assert.True(t, ok) {
				return
			}
			assert.Equal(t, "value1", v)
		})

		t.Run("full map", func(t *testing.T) {
			// Create a new Map32 with test data
			m := &Map32[string, string]{
				size:    32,
				entries: [32]StringMap32Entry[string, string]{},
			}

			err := m.Set("key2", "value2")
			assert.ErrorIs(t, err, ErrFullMap32)
		})
	})

}
