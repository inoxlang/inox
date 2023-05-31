package dom_ns

import (
	"bytes"
	"testing"

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
