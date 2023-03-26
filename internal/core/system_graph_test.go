package internal

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestObjectGraph(t *testing.T) {

	t.Run("object", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		NewGlobalState(ctx)

		graph := NewSystemGraph()

		object := NewObject()
		object.ProposeSystemGraph(graph)
		assert.Equal(t, 1, graph.nodes.Len())
		assert.NotNil(t, graph.nodes.At(ctx, 0))

		object.SetProp(ctx, "a", Int(1))
		assert.Len(t, graph.eventLog, 1)
		assert.Contains(t, graph.eventLog[0].text, "new prop")
	})

}
