package mapcoll

import (
	"testing"

	"github.com/inoxlang/inox/internal/core"
	"github.com/stretchr/testify/assert"
)

func TestMapIteration(t *testing.T) {

	t.Run("empty", func(t *testing.T) {
		ctx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		set := NewMapWithConfig(ctx, nil, MapConfig{})

		it := set.Iterator(ctx, core.IteratorConfiguration{})
		if !assert.False(t, it.HasNext(ctx)) {
			return
		}

		assert.False(t, it.Next(ctx))
	})

	t.Run("single entry", func(t *testing.T) {
		ctx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		set := NewMapWithConfig(ctx, core.NewWrappedValueList(INT_1, STRING_A), MapConfig{})

		it := set.Iterator(ctx, core.IteratorConfiguration{})
		if !assert.True(t, it.HasNext(ctx)) {
			return
		}

		assert.True(t, it.Next(ctx))
		assert.Equal(t, INT_1, it.Key(ctx))
		assert.Equal(t, STRING_A, it.Value(ctx))

		assert.False(t, it.HasNext(ctx))
		assert.False(t, it.Next(ctx))
	})

	t.Run("two entries", func(t *testing.T) {
		ctx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		set := NewMapWithConfig(ctx, core.NewWrappedValueList(INT_1, STRING_A, INT_2, STRING_B), MapConfig{})

		it := set.Iterator(ctx, core.IteratorConfiguration{})
		if !assert.True(t, it.HasNext(ctx)) {
			return
		}

		assert.True(t, it.Next(ctx))

		if INT_1 == it.Key(ctx).(core.Int) {
			assert.Equal(t, STRING_A, it.Value(ctx))

			assert.True(t, it.Next(ctx))
			assert.Equal(t, STRING_B, it.Value(ctx))
			assert.Equal(t, INT_2, it.Key(ctx))
		} else {
			assert.Equal(t, STRING_B, it.Value(ctx))
			assert.Equal(t, INT_2, it.Key(ctx))

			assert.True(t, it.Next(ctx))
			assert.Equal(t, STRING_A, it.Value(ctx))
			assert.Equal(t, INT_1, it.Key(ctx))
		}

		assert.False(t, it.HasNext(ctx))
		assert.False(t, it.Next(ctx))

	})

	t.Run("iteration in two goroutines", func(t *testing.T) {
		//TODO
	})

	t.Run("iteration as another goroutine modifies the Map", func(t *testing.T) {
		//TODO
	})

}
