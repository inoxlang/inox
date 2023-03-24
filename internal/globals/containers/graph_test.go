package internal

import (
	"testing"

	core "github.com/inox-project/inox/internal/core"
	"github.com/stretchr/testify/assert"
)

func TestCreateGraph(t *testing.T) {

	//whitebox testing

	t.Run("empty", func(t *testing.T) {
		ctx := core.NewContext(core.ContextConfig{})
		graph := NewGraph(ctx, core.NewWrappedValueList(), core.NewWrappedValueList())
		assert.Empty(t, graph.values)
		assert.Empty(t, graph.roots)
		assert.Zero(t, graph.graph.Nodes().Len())
		assert.Zero(t, graph.graph.Edges().Len())
	})

	t.Run("single node", func(t *testing.T) {
		ctx := core.NewContext(core.ContextConfig{})

		graph := NewGraph(ctx, core.NewWrappedValueList(core.Int(2)), core.NewWrappedValueList())

		assert.Equal(t, map[int64]core.Value{
			0: core.Int(2),
		}, graph.values)

		assert.Equal(t, map[int64]bool{
			0: true,
		}, graph.roots)

		assert.Equal(t, 1, graph.graph.Nodes().Len())
		assert.Zero(t, graph.graph.Edges().Len())
	})

	t.Run("two disconnected nodes", func(t *testing.T) {
		ctx := core.NewContext(core.ContextConfig{})

		graph := NewGraph(ctx, core.NewWrappedValueList(core.Int(2), core.Int(3)), core.NewWrappedValueList())

		assert.Equal(t, map[int64]core.Value{
			0: core.Int(2),
			1: core.Int(3),
		}, graph.values)

		assert.Equal(t, map[int64]bool{
			0: true,
			1: true,
		}, graph.roots)

		assert.Equal(t, 2, graph.graph.Nodes().Len())
		assert.Zero(t, graph.graph.Edges().Len())
	})

	t.Run("two connected nodes", func(t *testing.T) {
		ctx := core.NewContext(core.ContextConfig{})

		graph := NewGraph(ctx,
			core.NewWrappedValueList(core.Int(2), core.Int(3)),
			core.NewWrappedValueList(core.Int(0), core.Int(1)),
		)

		assert.Equal(t, map[int64]core.Value{
			0: core.Int(2),
			1: core.Int(3),
		}, graph.values)

		assert.Equal(t, map[int64]bool{
			0: true,
		}, graph.roots)

		assert.Equal(t, 2, graph.graph.Nodes().Len())
		assert.Equal(t, 1, graph.graph.Edges().Len())
	})
}
