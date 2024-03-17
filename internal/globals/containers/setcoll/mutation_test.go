package setcoll

import (
	"sync/atomic"
	"testing"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/containers/common"
	"github.com/stretchr/testify/assert"
)

func TestMutationNotSharedNoTx(t *testing.T) {
	t.Run("callback microtask should be called immediately after an element is added", func(t *testing.T) {
		ctx, _ := sharedSetTestSetup(t)
		defer ctx.CancelGracefully()

		pattern := NewSetPattern(SetConfig{
			Uniqueness: common.UniquenessConstraint{
				Type: common.UniqueRepr,
			},
		})

		set := NewSetWithConfig(ctx, nil, pattern.config)

		callCount := 0
		var mutation atomic.Value

		set.OnMutation(ctx, func(ctx *core.Context, m core.Mutation) (registerAgain bool) {
			registerAgain = true
			callCount++
			mutation.Store(m)
			return
		}, core.MutationWatchingConfiguration{
			Depth: core.ShallowWatching,
		})

		set.Add(ctx, INT_1)

		if assert.Equal(t, 1, callCount) {
			assert.Equal(t, NewAddElemMutation("/"), mutation.Load())
		}

		assert.True(t, bool(set.Has(ctx, INT_1)))
	})

	t.Run("callback microtask should be called immediately after an element is removed", func(t *testing.T) {
		ctx, _ := sharedSetTestSetup(t)
		defer ctx.CancelGracefully()

		pattern := NewSetPattern(SetConfig{
			Uniqueness: common.UniquenessConstraint{
				Type: common.UniqueRepr,
			},
		})

		set := NewSetWithConfig(ctx, nil, pattern.config)

		callCount := 0
		var mutation atomic.Value

		set.Add(ctx, INT_1)

		set.OnMutation(ctx, func(ctx *core.Context, m core.Mutation) (registerAgain bool) {
			registerAgain = true
			callCount++
			mutation.Store(m)
			return
		}, core.MutationWatchingConfiguration{
			Depth: core.ShallowWatching,
		})

		set.Remove(ctx, INT_1)

		if assert.Equal(t, 1, callCount) {
			assert.Equal(t, NewRemoveElemMutation("/"), mutation.Load())
		}

		assert.False(t, bool(set.Has(ctx, INT_1)))
	})

}

func TestMutationSharedNoTx(t *testing.T) {
	t.Run("callback microtask should be called immediately after an element is added", func(t *testing.T) {
		ctx, _ := sharedSetTestSetup(t)
		defer ctx.CancelGracefully()

		pattern := NewSetPattern(SetConfig{
			Uniqueness: common.UniquenessConstraint{
				Type: common.UniqueRepr,
			},
		})

		set := NewSetWithConfig(ctx, nil, pattern.config)
		set.Share(ctx.MustGetClosestState())

		callCount := 0
		var mutation atomic.Value

		set.OnMutation(ctx, func(ctx *core.Context, m core.Mutation) (registerAgain bool) {
			registerAgain = true
			callCount++
			mutation.Store(m)
			return
		}, core.MutationWatchingConfiguration{
			Depth: core.ShallowWatching,
		})

		set.Add(ctx, INT_1)
		if assert.Equal(t, 1, callCount) {
			assert.Equal(t, NewAddElemMutation("/"), mutation.Load())
		}

		assert.True(t, bool(set.Has(ctx, INT_1)))
	})

	t.Run("callback microtask should be called immediately after an element is removed", func(t *testing.T) {
		ctx, _ := sharedSetTestSetup(t)
		defer ctx.CancelGracefully()

		pattern := NewSetPattern(SetConfig{
			Uniqueness: common.UniquenessConstraint{
				Type: common.UniqueRepr,
			},
		})

		set := NewSetWithConfig(ctx, nil, pattern.config)
		set.Share(ctx.MustGetClosestState())

		callCount := 0
		var mutation atomic.Value

		set.Add(ctx, INT_1)

		set.OnMutation(ctx, func(ctx *core.Context, m core.Mutation) (registerAgain bool) {
			registerAgain = true
			callCount++
			mutation.Store(m)
			return
		}, core.MutationWatchingConfiguration{
			Depth: core.ShallowWatching,
		})

		set.Remove(ctx, INT_1)

		if assert.Equal(t, 1, callCount) {
			assert.Equal(t, NewRemoveElemMutation("/"), mutation.Load())
		}

		assert.False(t, bool(set.Has(ctx, INT_1)))
	})
}

func TestMutationSharedAndTx(t *testing.T) {
	t.Run("callback microtask should be called immediately after an element is added", func(t *testing.T) {
		ctx, _ := sharedSetTestSetup(t)
		defer ctx.CancelGracefully()

		tx := core.StartNewTransaction(ctx)

		pattern := NewSetPattern(SetConfig{
			Uniqueness: common.UniquenessConstraint{
				Type: common.UniqueRepr,
			},
		})

		set := NewSetWithConfig(ctx, nil, pattern.config)
		set.Share(ctx.MustGetClosestState())

		callCount := 0
		var mutation atomic.Value

		set.OnMutation(ctx, func(ctx *core.Context, m core.Mutation) (registerAgain bool) {
			registerAgain = true
			callCount++
			mutation.Store(m)
			return
		}, core.MutationWatchingConfiguration{
			Depth: core.ShallowWatching,
		})

		set.Add(ctx, INT_1)

		if assert.Equal(t, 1, callCount) {
			m := NewAddElemMutation("/")
			m.Tx = tx
			assert.Equal(t, m, mutation.Load())
		}

		assert.True(t, bool(set.Has(ctx, INT_1)))
	})

	t.Run("callback microtask should be called immediately after an element is removed", func(t *testing.T) {
		ctx, _ := sharedSetTestSetup(t)
		defer ctx.CancelGracefully()

		tx := core.StartNewTransaction(ctx)

		pattern := NewSetPattern(SetConfig{
			Uniqueness: common.UniquenessConstraint{
				Type: common.UniqueRepr,
			},
		})

		set := NewSetWithConfig(ctx, core.NewWrappedValueList(INT_1), pattern.config)
		set.Share(ctx.MustGetClosestState())

		callCount := 0
		var mutation atomic.Value

		set.OnMutation(ctx, func(ctx *core.Context, m core.Mutation) (registerAgain bool) {
			registerAgain = true
			callCount++
			mutation.Store(m)
			return
		}, core.MutationWatchingConfiguration{
			Depth: core.ShallowWatching,
		})

		set.Remove(ctx, INT_1)

		if assert.Equal(t, 1, callCount) {
			m := NewRemoveElemMutation("/")
			m.Tx = tx
			assert.Equal(t, m, mutation.Load())
		}

		assert.False(t, bool(set.Has(ctx, INT_1)))
	})
}
