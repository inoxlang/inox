package parse

import (
	"testing"

	"github.com/inoxlang/inox/internal/utils"
	"github.com/stretchr/testify/assert"
)

func TestGetNodeAtSpan(t *testing.T) {

	t.Run("shallow", func(t *testing.T) {
		chunk := utils.Must(ParseChunkSource(InMemorySource{
			NameString: "test",
			CodeString: "a = 1\na\nfn f(){}",
		}))

		span := chunk.GetLineColumnSingeCharSpan(1, 1)
		node, ok := chunk.GetNodeAtSpan(span)
		if !assert.True(t, ok) {
			return
		}
		assert.IsType(t, &IdentifierLiteral{}, node)

		span = chunk.GetLineColumnSingeCharSpan(2, 1)
		node, ok = chunk.GetNodeAtSpan(span)
		if !assert.True(t, ok) {
			return
		}
		assert.IsType(t, &IdentifierLiteral{}, node)
	})

	t.Run("deep (single line)", func(t *testing.T) {
		chunk := utils.Must(ParseChunkSource(InMemorySource{
			NameString: "test",
			CodeString: "fn f(arg %int){}",
		}))

		span := chunk.GetLineColumnSingeCharSpan(1, 10)
		node, ok := chunk.GetNodeAtSpan(span)
		if !assert.True(t, ok) {
			return
		}
		assert.IsType(t, &PatternIdentifierLiteral{}, node)

		span = chunk.GetLineColumnSingeCharSpan(1, 14)
		node, ok = chunk.GetNodeAtSpan(span)
		if !assert.True(t, ok) {
			return
		}
		assert.IsType(t, &PatternIdentifierLiteral{}, node)
	})
}
