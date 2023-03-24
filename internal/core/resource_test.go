package internal

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResourceMap(t *testing.T) {
	r := Path("/a")

	AcquireResource(r)
	assert.False(t, TryAcquireResource(r))

	ReleaseResource(r)
	assert.True(t, TryAcquireResource(r))
	ReleaseResource(r)
}
