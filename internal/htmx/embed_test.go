package htmx

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoad(t *testing.T) {

	Load()

	assert.NotEmpty(t, EXTENSIONS)
	//assert.NotEmpty(t, HEADERS.Request)
	assert.NotEmpty(t, HEADERS.Response)
}
