package htmx

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReadExtensions(t *testing.T) {

	ReadEmbedded()

	assert.NotEmpty(t, EXTENSIONS)
}
