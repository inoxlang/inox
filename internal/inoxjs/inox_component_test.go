package inoxjs

import (
	"context"
	"testing"

	"github.com/inoxlang/inox/internal/hyperscript/hscode"
	"github.com/stretchr/testify/assert"
)

func TestParseClientSideInterpolations(t *testing.T) {

	t.Run("empty", func(t *testing.T) {
		interpolations, err := ParseClientSideInterpolations(context.Background(), "", "")
		if !assert.NoError(t, err) {
			return
		}
		assert.Empty(t, interpolations)
	})

	t.Run("empty leading interpolation: length 0", func(t *testing.T) {
		interpolations, err := ParseClientSideInterpolations(context.Background(), "(())end", "(())end")
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
		assert.EqualValues(t, 0, interp.StartRuneIndex)
		assert.EqualValues(t, 4, interp.EndRuneIndex)
		assert.EqualValues(t, 2, interp.CodeStartRuneIndex)
		assert.EqualValues(t, 2, interp.CodeEndRuneIndex)
	})

	t.Run("empty leading interpolation: one space", func(t *testing.T) {
		interpolations, err := ParseClientSideInterpolations(context.Background(), "(( ))end", "(( ))end")
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
		assert.EqualValues(t, 0, interp.StartRuneIndex)
		assert.EqualValues(t, 5, interp.EndRuneIndex)
		assert.EqualValues(t, 2, interp.CodeStartRuneIndex)
		assert.EqualValues(t, 3, interp.CodeEndRuneIndex)
	})

	t.Run("non-leading interpolation", func(t *testing.T) {
		interpolations, err := ParseClientSideInterpolations(context.Background(), "((:count))end", "((:count))end")
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
		assert.EqualValues(t, 0, interp.StartRuneIndex)
		assert.EqualValues(t, 10, interp.EndRuneIndex)
		assert.EqualValues(t, 2, interp.CodeStartRuneIndex)
		assert.EqualValues(t, 8, interp.CodeEndRuneIndex)
	})

	t.Run("trailing interpolations", func(t *testing.T) {
		interpolations, err := ParseClientSideInterpolations(context.Background(), "start((:count))", "start((:count))")
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
		assert.EqualValues(t, 5, interp.StartRuneIndex)
		assert.EqualValues(t, 15, interp.EndRuneIndex)
		assert.EqualValues(t, 7, interp.CodeStartRuneIndex)
		assert.EqualValues(t, 13, interp.CodeEndRuneIndex)
	})

	t.Run("two interpolations", func(t *testing.T) {
		interpolations, err := ParseClientSideInterpolations(context.Background(), "((a))/((:count))", "((a))/((:count))")
		if !assert.NoError(t, err) {
			return
		}
		if !assert.Len(t, interpolations, 2) {
			return
		}

		//Check first interpolation.

		interp0 := interpolations[0]

		assert.Equal(t, "a", interp0.Expression)
		if !assert.Nil(t, interp0.ParsingError) {
			return
		}
		if !assert.NotNil(t, interp0.ParsingResult) {
			return
		}
		assert.True(t, hscode.IsSymbolWithName(interp0.ParsingResult.NodeData, "a"))

		assert.EqualValues(t, 0, interp0.StartRuneIndex)
		assert.EqualValues(t, 5, interp0.EndRuneIndex)
		assert.EqualValues(t, 2, interp0.CodeStartRuneIndex)
		assert.EqualValues(t, 3, interp0.CodeEndRuneIndex)

		//Check second interpolation.

		interp1 := interpolations[1]

		assert.Equal(t, ":count", interp1.Expression)
		if !assert.Nil(t, interp1.ParsingError) {
			return
		}
		if !assert.NotNil(t, interp1.ParsingResult) {
			return
		}
		assert.True(t, hscode.IsSymbolWithName(interp1.ParsingResult.NodeData, ":count"))

		assert.EqualValues(t, 6, interp1.StartRuneIndex)
		assert.EqualValues(t, 16, interp1.EndRuneIndex)
		assert.EqualValues(t, 8, interp1.CodeStartRuneIndex)
		assert.EqualValues(t, 14, interp1.CodeEndRuneIndex)
	})

	t.Run("JSON-encoded character before interpolation", func(t *testing.T) {
		interpolations, err := ParseClientSideInterpolations(context.Background(), "a((:count))end", `\u0061((:count))end`)
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
		assert.EqualValues(t, 6, interp.StartRuneIndex)
		assert.EqualValues(t, 16, interp.EndRuneIndex)
		assert.EqualValues(t, 8, interp.CodeStartRuneIndex)
		assert.EqualValues(t, 14, interp.CodeEndRuneIndex)
	})

	t.Run("JSON-encoded character in interpolation", func(t *testing.T) {
		interpolations, err := ParseClientSideInterpolations(context.Background(), `((:count))end`, `((:\u0063ount))end`)
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
		assert.EqualValues(t, 0, interp.StartRuneIndex)
		assert.EqualValues(t, 15, interp.EndRuneIndex)
		assert.EqualValues(t, 2, interp.CodeStartRuneIndex)
		assert.EqualValues(t, 13, interp.CodeEndRuneIndex)
	})
}
