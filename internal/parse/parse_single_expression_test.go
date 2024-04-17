package parse

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParsePath(t *testing.T) {

	t.Run("empty", func(t *testing.T) {
		p, ok := ParsePath("")
		assert.False(t, ok)
		assert.Empty(t, p)
	})
}

func TestParseURL(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		p, ok := ParseURL("")
		assert.False(t, ok)
		assert.Empty(t, p)
	})

	t.Run("invalid: scheme", func(t *testing.T) {
		p, ok := ParseURL("http://")
		assert.False(t, ok)
		assert.Empty(t, p)
	})

	t.Run("invalid: host", func(t *testing.T) {
		p, ok := ParseURL("http://example.com")
		assert.False(t, ok)
		assert.Empty(t, p)
	})

	t.Run("valid", func(t *testing.T) {
		p, ok := ParseURL("http://example.com/")
		assert.True(t, ok)
		assert.NotEmpty(t, p)
	})

	t.Run("valid", func(t *testing.T) {
		p, ok := ParseURL("http://example.com/?q=a")
		assert.True(t, ok)
		assert.NotEmpty(t, p)
	})
}
