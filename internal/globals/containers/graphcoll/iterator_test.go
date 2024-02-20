package graphcoll

import (
	"testing"

	"github.com/inoxlang/inox/internal/core"
	"github.com/stretchr/testify/assert"
)

func TestGraphIteration(t *testing.T) {

	t.Run("empty", func(t *testing.T) {
		ctx := core.NewContextWithEmptyState(core.ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		graph := NewGraph(ctx, core.NewWrappedValueList(), core.NewWrappedValueList())

		it := graph.Iterator(ctx, core.IteratorConfiguration{})
		if !assert.False(t, it.HasNext(ctx)) {
			return
		}

		assert.False(t, it.Next(ctx))
	})

	t.Run("single element", func(t *testing.T) {
		ctx := core.NewContextWithEmptyState(core.ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		graph := NewGraph(ctx, core.NewWrappedValueList(core.String("1")), core.NewWrappedValueList())

		it := graph.Iterator(ctx, core.IteratorConfiguration{})
		if !assert.True(t, it.HasNext(ctx)) {
			return
		}

		assert.True(t, it.Next(ctx))
		assert.Equal(t, core.Int(0), it.Key(ctx))

		v := it.Value(ctx)
		if !assert.IsType(t, (*GraphNode)(nil), v) {
			return
		}
		node := v.(*GraphNode)
		assert.Equal(t, &GraphNode{id: 0, graph: graph}, node)
		assert.Equal(t, core.String("1"), node.Prop(ctx, "data"))

		assert.False(t, it.HasNext(ctx))
		assert.False(t, it.Next(ctx))
	})

	t.Run("two elements", func(t *testing.T) {
		ctx := core.NewContextWithEmptyState(core.ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		graph := NewGraph(ctx, core.NewWrappedValueList(core.String("1"), core.String("2")), core.NewWrappedValueList())

		it := graph.Iterator(ctx, core.IteratorConfiguration{})
		if !assert.True(t, it.HasNext(ctx)) {
			return
		}

		assert.True(t, it.Next(ctx))
		assert.Equal(t, core.Int(0), it.Key(ctx))

		v := it.Value(ctx)
		if !assert.IsType(t, (*GraphNode)(nil), v) {
			return
		}
		node := v.(*GraphNode)

		if node.id == 0 {
			assert.Equal(t, core.String("1"), node.Prop(ctx, "data"))

			assert.True(t, it.Next(ctx))
			assert.Equal(t, core.Int(1), it.Key(ctx))

			v := it.Value(ctx)
			if !assert.IsType(t, (*GraphNode)(nil), v) {
				return
			}
			secondNode := v.(*GraphNode)
			assert.Equal(t, &GraphNode{id: 1, graph: graph}, secondNode)
			assert.Equal(t, core.String("2"), secondNode.Prop(ctx, "data"))
		} else {
			assert.Equal(t, core.String("2"), node.Prop(ctx, "data"))

			assert.True(t, it.Next(ctx))
			assert.Equal(t, core.Int(1), it.Key(ctx))

			v := it.Value(ctx)
			if !assert.IsType(t, (*GraphNode)(nil), v) {
				return
			}
			secondNode := v.(*GraphNode)
			assert.Equal(t, &GraphNode{id: 0, graph: graph}, secondNode)
			assert.Equal(t, core.String("1"), secondNode.Prop(ctx, "data"))
		}

		assert.False(t, it.HasNext(ctx))
		assert.False(t, it.Next(ctx))
	})

	t.Run("iteration in two goroutines", func(t *testing.T) {
		//TODO
	})

	t.Run("iteration as another goroutine modifies the Graph", func(t *testing.T) {
		//TODO
	})

}
