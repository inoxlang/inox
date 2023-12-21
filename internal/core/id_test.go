package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestULID(t *testing.T) {
	ulid1 := NewULID()

	parsed, err := ParseULID(ulid1.libValue().String())

	if !assert.NoError(t, err) {
		return
	}

	assert.Equal(t, ulid1, parsed)
}

func TestUUIDv4(t *testing.T) {
	uuid1 := NewUUIDv4()
	parsed, err := ParseULID(uuid1.libValue().String())

	if !assert.NoError(t, err) {
		return
	}

	assert.Equal(t, uuid1, parsed)
}
