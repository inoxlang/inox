package parse

import (
	"testing"

	"github.com/inoxlang/inox/internal/hyperscript/hscode"
	"github.com/inoxlang/inox/internal/hyperscript/hsparse"
	"github.com/stretchr/testify/assert"
)

func TestParseHyperscriptAttributeShorthand(t *testing.T) {

	t.Run("valid", func(t *testing.T) {
		chunk, err := parseChunkForgetTokens("h<div {on click toggle .red on me}></div>", "test", ParserOptions{
			ParseHyperscript: hsparse.ParseHyperScriptProgram,
		})

		if !assert.NoError(t, err) {
			return
		}

		attr := FindNode(chunk, (*HyperscriptAttributeShorthand)(nil), nil)
		if !assert.Nil(t, attr.HyperscriptParsingError) {
			return
		}
		assert.NotNil(t, attr.HyperscriptParsingResult)
	})

	t.Run("error", func(t *testing.T) {
		chunk, err := parseChunkForgetTokens("h<div {on click x .red on me}></div>", "test", ParserOptions{
			ParseHyperscript: hsparse.ParseHyperScriptProgram,
		})

		if !assert.Error(t, err) {
			return
		}

		attr := FindNode(chunk, (*HyperscriptAttributeShorthand)(nil), nil)
		if !assert.Nil(t, attr.HyperscriptParsingResult) {
			return
		}
		if !assert.NotNil(t, attr.HyperscriptParsingError) {
			return
		}

		aggregation, ok := err.(*ParsingErrorAggregation)
		if !assert.True(t, ok) {
			return
		}

		assert.Len(t, aggregation.Errors, 1)
		if !assert.Len(t, aggregation.ErrorPositions, 1) {
			return
		}

		pos := aggregation.ErrorPositions[0]
		assert.Equal(t, SourcePositionRange{
			SourceName:  "test",
			StartLine:   1,
			StartColumn: 17,
			EndLine:     1,
			EndColumn:   18,
			Span:        NodeSpan{16, 17},
		}, pos)

	})
}

func TestParseHyperscriptScriptElement(t *testing.T) {

	t.Run("valid", func(t *testing.T) {
		chunk, err := parseChunkForgetTokens("h<script h>on click toggle .red on me</script>", "test", ParserOptions{
			ParseHyperscript: hsparse.ParseHyperScriptProgram,
		})

		if !assert.NoError(t, err) {
			return
		}

		markupElement := FindNode(chunk, (*MarkupElement)(nil), nil)
		assert.IsType(t, (*hscode.ParsingResult)(nil), markupElement.RawElementParsingResult)
	})

	t.Run("error", func(t *testing.T) {
		chunk, err := parseChunkForgetTokens("h<script h>on click x .red on me</script>", "test", ParserOptions{
			ParseHyperscript: hsparse.ParseHyperScriptProgram,
		})

		if !assert.Error(t, err) {
			return
		}

		markupElement := FindNode(chunk, (*MarkupElement)(nil), nil)
		if !assert.NotNil(t, markupElement.RawElementParsingResult) {
			return
		}

		aggregation, ok := err.(*ParsingErrorAggregation)
		if !assert.True(t, ok) {
			return
		}

		assert.Len(t, aggregation.Errors, 1)
		if !assert.Len(t, aggregation.ErrorPositions, 1) {
			return
		}

		pos := aggregation.ErrorPositions[0]
		assert.Equal(t, SourcePositionRange{
			SourceName:  "test",
			StartLine:   1,
			StartColumn: 21,
			EndLine:     1,
			EndColumn:   22,
			Span:        NodeSpan{20, 21},
		}, pos)

	})
}
