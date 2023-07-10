package http_ns

import (
	"bytes"
	"testing"

	"github.com/inoxlang/inox/internal/core"
	"github.com/stretchr/testify/assert"
)

func TestCSPWrite(t *testing.T) {
	csp, err := NewCSPWithDirectives(nil)
	assert.NoError(t, err)

	b := bytes.NewBuffer(nil)
	csp.writeToBuf(b)

	assert.Equal(t,
		"connect-src 'self'; default-src 'none'; font-src 'self'; frame-ancestors 'none'; frame-src 'none';"+
			" img-src 'self'; script-src-elem 'self'; style-src 'self';", b.String())
}

func TestNewCSP(t *testing.T) {

	t.Run("string value", func(t *testing.T) {
		ctx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
		csp, err := NewCSP(ctx, core.NewObjectFromMap(core.ValMap{
			"default-src": core.Str("https://example.com"),
		}, ctx))

		if !assert.NoError(t, err) {
			return
		}

		assert.Contains(t, csp.String(), "default-src https://example.com")
	})

	t.Run("list value", func(t *testing.T) {
		ctx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
		csp, err := NewCSP(ctx, core.NewObjectFromMap(core.ValMap{
			"default-src": core.NewWrappedValueList(core.Str("https://example.com")),
		}, ctx))

		if !assert.NoError(t, err) {
			return
		}

		assert.Contains(t, csp.String(), "default-src https://example.com")
	})

}
