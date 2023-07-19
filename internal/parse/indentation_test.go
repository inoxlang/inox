package parse

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEstimateIndentationUnit(t *testing.T) {
	t.Run("single tab indentation", func(t *testing.T) {
		code := "manifest {\n\t{\n\t\t}\n\t}\n"
		node := MustParseChunk(code)

		assert.Equal(t, "\t", EstimateIndentationUnit([]rune(code), node))
	})

	t.Run("two spaces indentation", func(t *testing.T) {
		code := " manifest {\n  {\n    }\n  }\n"
		node := MustParseChunk(code)

		assert.Equal(t, "  ", EstimateIndentationUnit([]rune(code), node))
	})

	t.Run("three spaces indentation", func(t *testing.T) {
		code := "manifest {\n   {\n      }\n   }\n"
		node := MustParseChunk(code)

		assert.Equal(t, "   ", EstimateIndentationUnit([]rune(code), node))
	})

	t.Run("two tabs indentation", func(t *testing.T) {
		code := "manifest {\n\t\t{\n\t\t\t\t}\n\t\t}\n"
		node := MustParseChunk(code)

		assert.Equal(t, "\t\t", EstimateIndentationUnit([]rune(code), node))
	})

}
