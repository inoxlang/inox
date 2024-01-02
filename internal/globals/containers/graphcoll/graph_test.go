package graphcoll

import (
	"testing"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/memds"
	"github.com/stretchr/testify/assert"
)

func TestCreateGraph(t *testing.T) {

	//whitebox testing

	t.Run("empty", func(t *testing.T) {
		ctx := core.NewContext(core.ContextConfig{})
		graph := NewGraph(ctx, core.NewWrappedValueList(), core.NewWrappedValueList())
		assert.Empty(t, graph.roots)
		assert.Zero(t, graph.graph.NodeCount())
		assert.Zero(t, graph.graph.EdgeCount())
	})

	t.Run("single node", func(t *testing.T) {
		ctx := core.NewContext(core.ContextConfig{})

		graph := NewGraph(ctx, core.NewWrappedValueList(core.Int(2)), core.NewWrappedValueList())

		data, _ := graph.graph.NodeData(0)
		assert.Equal(t, core.Int(2), data)

		assert.Equal(t, map[memds.NodeId]bool{
			0: true,
		}, graph.roots)

		assert.Equal(t, 1, graph.graph.NodeCount())
		assert.Zero(t, graph.graph.EdgeCount())
	})

	t.Run("two disconnected nodes", func(t *testing.T) {
		ctx := core.NewContext(core.ContextConfig{})

		graph := NewGraph(ctx, core.NewWrappedValueList(core.Int(2), core.Int(3)), core.NewWrappedValueList())

		data, _ := graph.graph.NodeData(0)
		assert.Equal(t, core.Int(2), data)
		data, _ = graph.graph.NodeData(1)
		assert.Equal(t, core.Int(3), data)

		assert.Equal(t, map[memds.NodeId]bool{
			0: true,
			1: true,
		}, graph.roots)

		assert.Equal(t, 2, graph.graph.NodeCount())
		assert.Zero(t, graph.graph.EdgeCount())
	})

	t.Run("two connected nodes", func(t *testing.T) {
		ctx := core.NewContext(core.ContextConfig{})

		graph := NewGraph(ctx,
			core.NewWrappedValueList(core.Int(2), core.Int(3)),
			core.NewWrappedValueList(core.Int(0), core.Int(1)),
		)

		data, _ := graph.graph.NodeData(0)
		assert.Equal(t, core.Int(2), data)
		data, _ = graph.graph.NodeData(1)
		assert.Equal(t, core.Int(3), data)

		assert.Equal(t, map[memds.NodeId]bool{
			0: true,
		}, graph.roots)

		assert.Equal(t, 2, graph.graph.NodeCount())
		assert.Equal(t, int64(1), graph.graph.EdgeCount())
	})
}
