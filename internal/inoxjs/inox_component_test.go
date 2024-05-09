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
		assert.EqualValues(t, 2, interp.InnerStartRuneIndex)
		assert.EqualValues(t, 2, interp.InnerEndRuneIndex)
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
		assert.EqualValues(t, 2, interp.InnerStartRuneIndex)
		assert.EqualValues(t, 3, interp.InnerEndRuneIndex)
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
		assert.EqualValues(t, 2, interp.InnerStartRuneIndex)
		assert.EqualValues(t, 8, interp.InnerEndRuneIndex)
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
		assert.EqualValues(t, 7, interp.InnerStartRuneIndex)
		assert.EqualValues(t, 13, interp.InnerEndRuneIndex)
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
		assert.EqualValues(t, 2, interp0.InnerStartRuneIndex)
		assert.EqualValues(t, 3, interp0.InnerEndRuneIndex)

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
		assert.EqualValues(t, 8, interp1.InnerStartRuneIndex)
		assert.EqualValues(t, 14, interp1.InnerEndRuneIndex)
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
		assert.EqualValues(t, 8, interp.InnerStartRuneIndex)
		assert.EqualValues(t, 14, interp.InnerEndRuneIndex)
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
		assert.EqualValues(t, 2, interp.InnerStartRuneIndex)
		assert.EqualValues(t, 13, interp.InnerEndRuneIndex)
	})

	t.Run("leading space inside interpolation", func(t *testing.T) {
		interpolations, err := ParseClientSideInterpolations(context.Background(), "(( :count))", "(( :count))")
		if !assert.NoError(t, err) {
			return
		}
		if !assert.Len(t, interpolations, 1) {
			return
		}
		interp := interpolations[0]

		assert.Equal(t, " :count", interp.Expression)
		if !assert.Nil(t, interp.ParsingError) {
			return
		}
		if !assert.NotNil(t, interp.ParsingResult) {
			return
		}
		if !assert.True(t, hscode.IsSymbolWithName(interp.ParsingResult.NodeData, ":count")) {
			return
		}

		start, end := hscode.GetNodeSpan(interp.ParsingResult.NodeData)
		assert.EqualValues(t, 1, start)
		assert.EqualValues(t, 7, end)

		assert.EqualValues(t, 0, interp.StartRuneIndex)
		assert.EqualValues(t, 11, interp.EndRuneIndex)
		assert.EqualValues(t, 2, interp.InnerStartRuneIndex)
		assert.EqualValues(t, 9, interp.InnerEndRuneIndex)
	})

	t.Run("trailing space inside interpolation", func(t *testing.T) {
		interpolations, err := ParseClientSideInterpolations(context.Background(), "((:count ))", "((:count ))")
		if !assert.NoError(t, err) {
			return
		}
		if !assert.Len(t, interpolations, 1) {
			return
		}
		interp := interpolations[0]

		assert.Equal(t, ":count ", interp.Expression)
		if !assert.Nil(t, interp.ParsingError) {
			return
		}
		if !assert.NotNil(t, interp.ParsingResult) {
			return
		}
		assert.True(t, hscode.IsSymbolWithName(interp.ParsingResult.NodeData, ":count"))
		assert.EqualValues(t, 0, interp.StartRuneIndex)
		assert.EqualValues(t, 11, interp.EndRuneIndex)
		assert.EqualValues(t, 2, interp.InnerStartRuneIndex)
		assert.EqualValues(t, 9, interp.InnerEndRuneIndex)
	})
}
