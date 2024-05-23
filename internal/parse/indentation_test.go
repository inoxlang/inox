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

	t.Run("single tab indentation in for statement", func(t *testing.T) {
		code := "manifest {}\nfor []{\ta = 1\n}"
		node := MustParseChunk(code)

		assert.Equal(t, "\t", EstimateIndentationUnit([]rune(code), node))
	})

	t.Run("two-tab indentation in for statement", func(t *testing.T) {
		code := "manifest {}\nfor []{\t\ta = 1\n}"
		node := MustParseChunk(code)

		assert.Equal(t, "\t\t", EstimateIndentationUnit([]rune(code), node))
	})

	t.Run("single tab indentation in switch statement", func(t *testing.T) {
		code := "manifest {}\nswitch 1{\t1 {}\n}"
		node := MustParseChunk(code)

		assert.Equal(t, "\t", EstimateIndentationUnit([]rune(code), node))
	})

	t.Run("two-tab indentation in switch statement", func(t *testing.T) {
		code := "manifest {}\nswitch 1{\t\t1 {}\n}"
		node := MustParseChunk(code)

		assert.Equal(t, "\t\t", EstimateIndentationUnit([]rune(code), node))
	})

	t.Run("single tab indentation in match statement", func(t *testing.T) {
		code := "manifest {}\nmatch 1{\t1 {}\n}"
		node := MustParseChunk(code)

		assert.Equal(t, "\t", EstimateIndentationUnit([]rune(code), node))
	})

	t.Run("two-tab indentation in switch statement", func(t *testing.T) {
		code := "manifest {}\nmatch 1{\t\t1 {}\n}"
		node := MustParseChunk(code)

		assert.Equal(t, "\t\t", EstimateIndentationUnit([]rune(code), node))
	})

}
