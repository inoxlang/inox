package internal

import (
	"testing"

	"github.com/inoxlang/inox/internal/utils"
	"github.com/stretchr/testify/assert"
)

func TestGetNodeAtSpan(t *testing.T) {

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
}
