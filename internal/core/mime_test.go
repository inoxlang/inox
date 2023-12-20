package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMime(t *testing.T) {

	t.Run("params", func(t *testing.T) {
		mimeType := Mimetype("text/css; charset=utf-8")
		assert.EqualValues(t, "text/css", mimeType.WithoutParams())
	})

	t.Run("params: no space before params", func(t *testing.T) {
		mimeType := Mimetype("text/css;charset=utf-8")
		assert.EqualValues(t, "text/css", mimeType.WithoutParams())
	})

}
