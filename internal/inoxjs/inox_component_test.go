package inoxjs

import (
	"context"
	"testing"

	"github.com/inoxlang/inox/internal/hyperscript/hscode"
	"github.com/stretchr/testify/assert"
)

func TestParseClientSideInterpolations(t *testing.T) {

	t.Run("empty", func(t *testing.T) {
		interpolations, err := ParseClientSideInterpolations(context.Background(), "")
		if !assert.NoError(t, err) {
			return
		}
		assert.Empty(t, interpolations)
	})

	t.Run("empty leading interpolation: length 0", func(t *testing.T) {
		interpolations, err := ParseClientSideInterpolations(context.Background(), "(())end")
		if !assert.NoError(t, err) {
			return
		}
		if !assert.Len(t, interpolations, 1) {
			return
		}
		interp := interpolations[0]

		assert.Empty(t, interp.Expression)
		if !assert.NotNil(t, interp.ParsingError) {
			return
		}
		assert.Nil(t, interp.ParsingResult)
	})

	t.Run("empty leading interpolation: one space", func(t *testing.T) {
		interpolations, err := ParseClientSideInterpolations(context.Background(), "(( ))end")
		if !assert.NoError(t, err) {
			return
		}
		if !assert.Len(t, interpolations, 1) {
			return
		}
		interp := interpolations[0]

		assert.Equal(t, " ", interp.Expression)
		if !assert.NotNil(t, interp.ParsingError) {
			return
		}
		assert.Nil(t, interp.ParsingResult)
	})

	t.Run("non-empty leading interpolation", func(t *testing.T) {
		interpolations, err := ParseClientSideInterpolations(context.Background(), "((:count))end")
		if !assert.NoError(t, err) {
			return
		}
		if !assert.Len(t, interpolations, 1) {
			return
		}
		interp := interpolations[0]

		assert.Equal(t, ":count", interp.Expression)
		if !assert.Nil(t, interp.ParsingError) {
			return
		}
		if !assert.NotNil(t, interp.ParsingResult) {
			return
		}
		assert.True(t, hscode.IsSymbolWithName(interp.ParsingResult.NodeData, ":count"))
	})

	t.Run("non-empty trailing interpolations", func(t *testing.T) {
		interpolations, err := ParseClientSideInterpolations(context.Background(), "start((:count))")
		if !assert.NoError(t, err) {
			return
		}
		if !assert.Len(t, interpolations, 1) {
			return
		}
		interp := interpolations[0]

		assert.Equal(t, ":count", interp.Expression)
		if !assert.Nil(t, interp.ParsingError) {
			return
		}
		if !assert.NotNil(t, interp.ParsingResult) {
			return
		}
		assert.True(t, hscode.IsSymbolWithName(interp.ParsingResult.NodeData, ":count"))
	})

	t.Run("two interpolations", func(t *testing.T) {
		interpolations, err := ParseClientSideInterpolations(context.Background(), "((a))/((:count))")
		if !assert.NoError(t, err) {
			return
		}
		if !assert.Len(t, interpolations, 2) {
			return
		}

		interp0 := interpolations[0]

		assert.Equal(t, "a", interp0.Expression)
		if !assert.Nil(t, interp0.ParsingError) {
			return
		}
		if !assert.NotNil(t, interp0.ParsingResult) {
			return
		}
		assert.True(t, hscode.IsSymbolWithName(interp0.ParsingResult.NodeData, "a"))

		interp2 := interpolations[1]

		assert.Equal(t, ":count", interp2.Expression)
		if !assert.Nil(t, interp2.ParsingError) {
			return
		}
		if !assert.NotNil(t, interp2.ParsingResult) {
			return
		}
		assert.True(t, hscode.IsSymbolWithName(interp2.ParsingResult.NodeData, ":count"))
	})

}
