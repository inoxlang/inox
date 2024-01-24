package core

import (
	"testing"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/assert"
)

func TestULID(t *testing.T) {
	t.Parallel()
	t.Run("base case", func(t *testing.T) {
		id := NewULID()

		parsed, err := ParseULID(id.libValue().String())

		if !assert.NoError(t, err) {
			return
		}

		assert.Equal(t, id, parsed)
	})

	t.Run("monotonic", func(t *testing.T) {
		if testing.Short() {
			t.Skip()
		}
		for i := 0; i < 100_000; i++ {

			ulid1 := NewULID()
			ulid2 := NewULID()

			//ulid2 should be > ulid1
			if !assert.Equalf(t, 1, ulid2.libValue().Compare(ulid.ULID(ulid1)), "%s <= %s", ulid2, ulid1) {
				return
			}
		}
	})

	t.Run("MAX_ULID", func(t *testing.T) {
		assert.True(t, MAX_ULID.After(MIN_ULID))
		assert.True(t, MAX_ULID.After(NewULID()))

		assert.False(t, MAX_ULID.After(MAX_ULID))
		assert.False(t, MAX_ULID.Before(MAX_ULID))
	})

	t.Run("MIN_ULID", func(t *testing.T) {
		assert.True(t, MIN_ULID.Before(MAX_ULID))
		assert.True(t, MIN_ULID.Before(NewULID()))

		assert.False(t, MIN_ULID.After(MIN_ULID))
		assert.False(t, MIN_ULID.Before(MIN_ULID))
	})
}

func TestUUIDv4(t *testing.T) {
	uuid1 := NewUUIDv4()
	parsed, err := ParseUUIDv4(uuid1.libValue().String())

	if !assert.NoError(t, err) {
		return
	}

	assert.Equal(t, uuid1, parsed)
}
