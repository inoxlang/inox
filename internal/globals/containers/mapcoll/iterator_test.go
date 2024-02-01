package mapcoll

import (
	"strconv"
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

	t.Run("iteration should be thread safe", func(t *testing.T) {
		ctx1, ctx2, _ := sharedMapTestSetup2(t)
		defer ctx1.CancelGracefully()
		defer ctx2.CancelGracefully()

		core.StartNewReadonlyTransaction(ctx1)
		//ctx2 has no transaction on purpose.

		m := NewMapWithConfig(ctx1, core.NewWrappedValueList(INT_1, STRING_A, INT_2, STRING_B), MapConfig{})

		m.Share(ctx1.GetClosestState())

		const ADD_COUNT = 10_000

		done := make(chan struct{})
		go func() {
			for i := 0; i < ADD_COUNT; i++ {
				m.Set(ctx2, core.Int(i+5), core.String(strconv.Itoa(i)))
			}
			done <- struct{}{}
		}()

		callCount := 0

	loop:
		for {
			select {
			case <-done:
				break loop
			default:
				it := m.Iterator(ctx1, core.IteratorConfiguration{})

				for it.Next(ctx1) {
					callCount++
				}
			}
		}

		assert.Greater(t, callCount, ADD_COUNT/10) //just make sure the function was called several times.
	})

}
