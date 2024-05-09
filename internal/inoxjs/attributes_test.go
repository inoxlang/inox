package inoxjs

import (
	"context"
	"testing"

	"github.com/inoxlang/inox/internal/parse"
	"github.com/stretchr/testify/assert"
)

func TestAnalyzeInoxJSAttributes(t *testing.T) {
	ctx := context.Background()

	t.Run("no attributes", func(t *testing.T) {
		chunk := parse.MustParseChunkSource(parse.InMemorySource{
			NameString: "test",
			CodeString: "html<div></div>",
		})

		markupElem := parse.FindFirstNode(chunk.Node, (*parse.MarkupElement)(nil))
		isComponent, introducedElementScopedVarNames, errors, criticalErr := AnalyzeInoxJsAttributes(ctx, markupElem, chunk)
		if !assert.NoError(t, criticalErr) {
			return
		}
		assert.Empty(t, errors)
		assert.Empty(t, introducedElementScopedVarNames)
		assert.False(t, isComponent)
	})

	t.Run("x-for attribute with valid value", func(t *testing.T) {
		chunk := parse.MustParseChunkSource(parse.InMemorySource{
			NameString: "test",
			CodeString: "html<div x-for=`:e in :list`></div>",
		})

		markupElem := parse.FindFirstNode(chunk.Node, (*parse.MarkupElement)(nil))
		isComponent, introducedElementScopedVarNames, errors, criticalErr := AnalyzeInoxJsAttributes(ctx, markupElem, chunk)
		if !assert.NoError(t, criticalErr) {
			return
		}
		assert.Empty(t, errors)
		assert.Equal(t, []string{":e"}, introducedElementScopedVarNames)
		assert.True(t, isComponent)
	})

	t.Run("x-for attribute with invalid value", func(t *testing.T) {
		chunk := parse.MustParseChunkSource(parse.InMemorySource{
			NameString: "test",
			CodeString: "html<div x-for=`:e`></div>",
		})

		markupElem := parse.FindFirstNode(chunk.Node, (*parse.MarkupElement)(nil))
		isComponent, introducedElementScopedVarNames, errors, criticalErr := AnalyzeInoxJsAttributes(ctx, markupElem, chunk)
		if !assert.NoError(t, criticalErr) {
			return
		}
		assert.Equal(t, []Error{
			MakeError(INVALID_VALUE_FOR_FOR_LOOP_ATTR, chunk.GetSourcePosition(parse.NodeSpan{Start: 15, End: 19})),
		}, errors)
		assert.Empty(t, introducedElementScopedVarNames)
		assert.True(t, isComponent)
	})

	t.Run("x-if attribute with valid value", func(t *testing.T) {
		chunk := parse.MustParseChunkSource(parse.InMemorySource{
			NameString: "test",
			CodeString: "html<div x-if=`:show`></div>",
		})

		markupElem := parse.FindFirstNode(chunk.Node, (*parse.MarkupElement)(nil))
		isComponent, introducedElementScopedVarNames, errors, criticalErr := AnalyzeInoxJsAttributes(ctx, markupElem, chunk)
		if !assert.NoError(t, criticalErr) {
			return
		}
		assert.Empty(t, errors)
		assert.Empty(t, introducedElementScopedVarNames)
		assert.False(t, isComponent)
	})

	t.Run("x-if attribute with Hyperscript parsing error", func(t *testing.T) {
		chunk := parse.MustParseChunkSource(parse.InMemorySource{
			NameString: "test",
			CodeString: "html<div x-if=`:`></div>",
		})

		markupElem := parse.FindFirstNode(chunk.Node, (*parse.MarkupElement)(nil))
		isComponent, introducedElementScopedVarNames, errors, criticalErr := AnalyzeInoxJsAttributes(ctx, markupElem, chunk)
		if !assert.NoError(t, criticalErr) {
			return
		}
		if assert.Len(t, errors, 1) {
			err := errors[0]
			assert.True(t, err.IsHyperscriptParsingError)
		}
		assert.Empty(t, introducedElementScopedVarNames)
		assert.False(t, isComponent)
	})
}
