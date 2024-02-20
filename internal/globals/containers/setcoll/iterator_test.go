package setcoll

import (
	"strconv"
	"testing"
	"time"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/containers/common"
	"github.com/stretchr/testify/assert"
)

func TestSetIteration(t *testing.T) {

	t.Run("empty", func(t *testing.T) {
		ctx := core.NewContextWithEmptyState(core.ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		set := NewSetWithConfig(ctx, nil, SetConfig{
			Element: core.ANYVAL_PATTERN,
			Uniqueness: common.UniquenessConstraint{
				Type: common.UniqueRepr,
			},
		})

		it := set.Iterator(ctx, core.IteratorConfiguration{})
		if !assert.False(t, it.HasNext(ctx)) {
			return
		}

		assert.False(t, it.Next(ctx))
	})

	t.Run("single element", func(t *testing.T) {
		ctx := core.NewContextWithEmptyState(core.ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		set := NewSetWithConfig(ctx, core.NewWrappedValueList(core.Int(1)), SetConfig{
			Element: core.INT_PATTERN,
			Uniqueness: common.UniquenessConstraint{
				Type: common.UniqueRepr,
			},
		})

		it := set.Iterator(ctx, core.IteratorConfiguration{})
		if !assert.True(t, it.HasNext(ctx)) {
			return
		}

		assert.True(t, it.Next(ctx))
		assert.Equal(t, core.Int(1), it.Value(ctx))
		assert.Equal(t, core.String("1"), it.Key(ctx))

		assert.False(t, it.HasNext(ctx))
		assert.False(t, it.Next(ctx))
	})

	t.Run("two elements", func(t *testing.T) {
		ctx := core.NewContextWithEmptyState(core.ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		set := NewSetWithConfig(ctx, core.NewWrappedValueList(core.Int(1), core.Int(2)), SetConfig{
			Element: core.INT_PATTERN,
			Uniqueness: common.UniquenessConstraint{
				Type: common.UniqueRepr,
			},
		})

		it := set.Iterator(ctx, core.IteratorConfiguration{})
		if !assert.True(t, it.HasNext(ctx)) {
			return
		}

		assert.True(t, it.Next(ctx))

		if core.Int(1) == it.Value(ctx).(core.Int) {
			assert.Equal(t, core.String("1"), it.Key(ctx))

			assert.True(t, it.Next(ctx))
			assert.Equal(t, core.Int(2), it.Value(ctx))
			assert.Equal(t, core.String("2"), it.Key(ctx))
		} else {
			assert.Equal(t, core.Int(2), it.Value(ctx))
			assert.Equal(t, core.String("2"), it.Key(ctx))

			assert.True(t, it.Next(ctx))
			assert.Equal(t, core.Int(1), it.Value(ctx))
			assert.Equal(t, core.String("1"), it.Key(ctx))
		}

		assert.False(t, it.HasNext(ctx))
		assert.False(t, it.Next(ctx))

	})

	t.Run("iteration in two goroutines", func(t *testing.T) {
		var elements []core.Serializable
		for i := 0; i < 100_000; i++ {
			elements = append(elements, core.Int(i))
		}
		tuple := core.NewTuple(elements)

		go func() {
			ctx := core.NewContextWithEmptyState(core.ContextConfig{}, nil)
			defer ctx.CancelGracefully()

			set := NewSetWithConfig(ctx, tuple, SetConfig{
				Element: core.INT_PATTERN,
				Uniqueness: common.UniquenessConstraint{
					Type: common.UniqueRepr,
				},
			})

			it := set.Iterator(ctx, core.IteratorConfiguration{})

			for it.HasNext(ctx) {
				it.Next(ctx)
				_ = it.Value(ctx)
			}
		}()

		time.Sleep(time.Microsecond)

		ctx := core.NewContextWithEmptyState(core.ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		set := NewSetWithConfig(ctx, tuple, SetConfig{
			Element: core.INT_PATTERN,
			Uniqueness: common.UniquenessConstraint{
				Type: common.UniqueRepr,
			},
		})

		it := set.Iterator(ctx, core.IteratorConfiguration{})

		i := 0
		for it.HasNext(ctx) {
			assert.True(t, it.Next(ctx))
			val := it.Value(ctx).(core.Int)
			stringifiedVal := core.String(strconv.Itoa(int(val)))

			if !assert.Equal(t, it.Key(ctx), stringifiedVal) {
				return
			}
			i++
		}
		assert.Equal(t, 100_000, i)
	})

	t.Run("iteration as another goroutine modifies the Set", func(t *testing.T) {
		var elements []core.Serializable
		for i := 0; i < 100_000; i++ {
			elements = append(elements, core.Int(i))
		}
		tuple := core.NewTuple(elements)

		go func() {
			ctx := core.NewContextWithEmptyState(core.ContextConfig{}, nil)
			defer ctx.CancelGracefully()

			set := NewSetWithConfig(ctx, tuple, SetConfig{
				Element: core.INT_PATTERN,
				Uniqueness: common.UniquenessConstraint{
					Type: common.UniqueRepr,
				},
			})

			for i := 100_000; i < 200_000; i++ {
				set.Add(ctx, core.Int(i))
			}
		}()

		time.Sleep(time.Microsecond)

		for index := 0; index < 5; index++ {

			ctx := core.NewContextWithEmptyState(core.ContextConfig{}, nil)
			defer ctx.CancelGracefully()

			set := NewSetWithConfig(ctx, tuple, SetConfig{
				Element: core.INT_PATTERN,
				Uniqueness: common.UniquenessConstraint{
					Type: common.UniqueRepr,
				},
			})

			it := set.Iterator(ctx, core.IteratorConfiguration{})

			i := 0
			for it.HasNext(ctx) {
				if !assert.True(t, it.Next(ctx)) {
					return
				}

				val := it.Value(ctx).(core.Int)
				stringifiedVal := core.String(strconv.Itoa(int(val)))

				if !assert.Equal(t, it.Key(ctx), stringifiedVal) {
					return
				}
				i++
			}
			if !assert.Equal(t, 100_000, i) {
				return
			}
		}
	})

	t.Run("iteration should be thread safe", func(t *testing.T) {
		ctx1, ctx2, _ := sharedSetTestSetup2(t)
		defer ctx1.CancelGracefully()
		defer ctx2.CancelGracefully()

		core.StartNewReadonlyTransaction(ctx1)
		//ctx2 has no transaction on purpose.

		elements := core.NewWrappedValueList(INT_1, INT_2)

		set := NewSetWithConfig(ctx1, elements, SetConfig{
			Uniqueness: common.UniquenessConstraint{
				Type: common.UniqueRepr,
			},
		})

		set.Share(ctx1.GetClosestState())

		const ADD_COUNT = 10_000

		done := make(chan struct{})
		go func() {
			for i := 0; i < ADD_COUNT; i++ {
				set.Add(ctx2, core.Int(i+5))
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
				it := set.Iterator(ctx1, core.IteratorConfiguration{})

				for it.Next(ctx1) {
					callCount++
				}
			}
		}

		assert.Greater(t, callCount, ADD_COUNT/10) //just make sure the function was called several times.
	})
}
