package http_ns

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStatus(t *testing.T) {
	s, err := ParseStatus("")
	assert.Error(t, err)
	assert.Zero(t, s)

	s, err = ParseStatus(" ")
	assert.Error(t, err)
	assert.Zero(t, s)

	s, err = ParseStatus("200")
	assert.Error(t, err)
	assert.Zero(t, s)

	s, err = ParseStatus("200 OK")
	assert.NoError(t, err)
	assert.Equal(t, Status{code: 200, reasonPhrase: "OK", text: "200 OK"}, s)

	s, err = ParseStatus("404 Not Found")
	assert.NoError(t, err)
	assert.Equal(t, Status{code: 404, reasonPhrase: "Not Found", text: "404 Not Found"}, s)
}
