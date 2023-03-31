package internal

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestObjectGraph(t *testing.T) {

	t.Run("object should add and event when it's mutated", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		NewGlobalState(ctx)

		graph := NewSystemGraph()

		object := NewObject()
		object.ProposeSystemGraph(ctx, graph, "", nil)
		assert.Len(t, graph.nodes.list, 1)
		assert.NotNil(t, graph.nodes.list[0])

		object.SetProp(ctx, "a", Int(1))
		assert.Len(t, graph.eventLog, 1)
		assert.Contains(t, graph.eventLog[0].text, "new prop")

	})

	t.Run("object should add a child node for each of its subsystems", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		state := NewGlobalState(ctx)

		graph := NewSystemGraph()

		object := NewObjectFromMap(ValMap{
			"0": createTestLifetimeJob(t, state, ""),
			"inner0": NewObjectFromMap(ValMap{
				"0": createTestLifetimeJob(t, state, ""),
			}, ctx),
			"inner1": NewObjectFromMap(ValMap{
				"0": createTestLifetimeJob(t, state, ""),
			}, ctx),
		}, ctx)
		object.ProposeSystemGraph(ctx, graph, "", nil)

		assert.Len(t, graph.nodes.list, 3)
		assert.NotNil(t, graph.nodes.list[0])
		assert.NotNil(t, graph.nodes.list[1])
		assert.NotNil(t, graph.nodes.list[2])
	})

	t.Run("AddSystemGraphEvent", func(t *testing.T) {
		ctx := NewContext(ContextConfig{})
		NewGlobalState(ctx)

		graph := NewSystemGraph()

		object := NewObject()
		object.ProposeSystemGraph(ctx, graph, "", nil)

		object.AddSystemGraphEvent(ctx, "an event")
		assert.Len(t, graph.eventLog, 1)
		assert.Contains(t, graph.eventLog[0].text, "an event")
	})

}

func TestSystemGraph(t *testing.T) {
	ctx := NewContext(ContextConfig{})
	NewGlobalState(ctx)

	val1 := NewObject()
	val2 := NewObject()
	graph := NewSystemGraph()

	graph.AddNode(ctx, val1, "a")
	graph.AddWatchedNode(ctx, val1, val2, "")

	if !assert.Len(t, graph.nodes.list, 2) {
		return
	}
	assert.NotNil(t, graph.nodes.list[0])
	assert.NotNil(t, graph.nodes.list[1])

	node := graph.nodes.list[0]
	assert.Len(t, node.edgesFrom, 1)
	assert.Equal(t, EdgeWatched, node.edgesFrom[0].kind)
}
