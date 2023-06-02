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

func TestHost(t *testing.T) {
	t.Run("HostWithoutPort", func(t *testing.T) {
		assert.Equal(t, Host("https://127.0.0.1"), Host("https://127.0.0.1").HostWithoutPort())
		assert.Equal(t, Host("https://127.0.0.1"), Host("https://127.0.0.1:80").HostWithoutPort())
		assert.Equal(t, Host("https://[::1]"), Host("https://[::1]").HostWithoutPort()) //stable if not port
		assert.Equal(t, Host("https://::1"), Host("https://[::1]:80").HostWithoutPort())
		assert.Equal(t, Host("https://::1"), Host("https://[::1]:1").HostWithoutPort())
	})
}
