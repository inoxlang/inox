package setcoll

import (
	"testing"

	"github.com/inoxlang/inox/internal/core"
	"github.com/stretchr/testify/assert"

	"github.com/inoxlang/inox/internal/globals/containers/common"
)

func TestSharedUnpersistedSetAdd(t *testing.T) {
	t.Run("Set should be updated at end of transaction if .Add was called transactionnaly", func(t *testing.T) {
		ctx, _ := sharedSetTestSetup(t)
		defer ctx.CancelGracefully()

		tx := core.StartNewTransaction(ctx)

		pattern := NewSetPattern(SetConfig{
			Uniqueness: common.UniquenessConstraint{
				Type: common.UniqueRepr,
			},
		})

		set := NewSetWithConfig(ctx, nil, pattern.config)
		set.Add(ctx, core.Int(1))

		assert.NoError(t, tx.Commit(ctx))

		otherCtx, _ := sharedSetTestSetup(t)
		defer ctx.CancelGracefully()

		assert.True(t, bool(set.Has(otherCtx, core.Int(1))))

	})

	t.Run("adding an element to a URL-based uniqueness shared Set with no storage should cause a panic", func(t *testing.T) {
		ctx, _ := sharedSetTestSetup(t)
		defer ctx.CancelGracefully()

		pattern := NewSetPattern(SetConfig{
			Uniqueness: common.UniquenessConstraint{Type: common.UniqueURL},
		})

		set := NewSetWithConfig(ctx, nil, pattern.config)
		set.Share(ctx.GetClosestState())

		obj := core.NewObjectFromMap(core.ValMap{}, ctx)

		assert.PanicsWithValue(t, ErrURLUniquenessOnlySupportedIfPersistedSharedSet, func() {
			set.Add(ctx, obj)
		})
	})

	//Tests with several transactions.

	t.Run("transactions should wait for the previous transaction to finish", func(t *testing.T) {
		ctx1 := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
		defer ctx1.CancelGracefully()
		tx1 := core.StartNewTransaction(ctx1)

		ctx2 := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
		defer ctx2.CancelGracefully()
		core.StartNewTransaction(ctx2)

		set := NewSetWithConfig(ctx1, nil, SetConfig{
			Element: core.INT_PATTERN,
			Uniqueness: common.UniquenessConstraint{
				Type: common.UniqueRepr,
			},
		})
		set.Share(ctx1.GetClosestState())

		set.Add(ctx1, core.Int(1))
		assert.True(t, bool(set.Has(ctx1, core.Int(1))))

		tx2Done := make(chan struct{})
		go func() { //second transaction
			set.Add(ctx2, core.Int(2))

			//since the first transaction should be finished,
			//the other element should have been added.
			assert.True(t, bool(set.Has(ctx2, core.Int(1))))
			assert.True(t, bool(set.Has(ctx2, core.Int(2))))
			tx2Done <- struct{}{}
		}()

		assert.NoError(t, tx1.Commit(ctx1))

		<-tx2Done
	})

	t.Run("adding an element with the same property value as another element is not allowed", func(t *testing.T) {
		ctx, _ := sharedSetTestSetup(t)
		defer ctx.CancelGracefully()

		pattern := NewSetPattern(SetConfig{
			Uniqueness: common.UniquenessConstraint{
				Type:         common.UniquePropertyValue,
				PropertyName: "name",
			},
		})

		set := NewSetWithConfig(ctx, nil, pattern.config)
		set.Share(ctx.GetClosestState())

		obj1 := core.NewObjectFromMap(core.ValMap{"name": core.Str("a")}, ctx)
		obj2 := core.NewObjectFromMap(core.ValMap{"name": core.Str("a")}, ctx)
		set.Add(ctx, obj1)

		func() {
			defer func() {
				e := recover()
				if !assert.NotNil(t, e) {
					return
				}
				assert.ErrorIs(t, e.(error), ErrCannotAddDifferentElemWithSamePropertyValue)
			}()

			set.Add(ctx, obj2)
		}()
	})

}

func TestSharedUnpersistedSetHas(t *testing.T) {

	t.Run("checking the existence of an element of a URL-based uniqueness shared Set with no storage should cause a panic", func(t *testing.T) {
		ctx, _ := sharedSetTestSetup(t)
		defer ctx.CancelGracefully()

		pattern := NewSetPattern(SetConfig{
			Uniqueness: common.UniquenessConstraint{Type: common.UniqueURL},
		})

		set := NewSetWithConfig(ctx, nil, pattern.config)
		set.Share(ctx.GetClosestState())

		obj := core.NewObjectFromMap(core.ValMap{}, ctx)

		assert.PanicsWithValue(t, ErrURLUniquenessOnlySupportedIfPersistedSharedSet, func() {
			set.Has(ctx, obj)
		})
	})

	t.Run("an element with the same property value as another element is not considered to be in the set", func(t *testing.T) {
		ctx, _ := sharedSetTestSetup(t)
		defer ctx.CancelGracefully()

		pattern := NewSetPattern(SetConfig{
			Uniqueness: common.UniquenessConstraint{
				Type:         common.UniquePropertyValue,
				PropertyName: "name",
			},
		})

		set := NewSetWithConfig(ctx, nil, pattern.config)
		set.Share(ctx.GetClosestState())

		obj1 := core.NewObjectFromMap(core.ValMap{"name": core.Str("a")}, ctx)
		obj2 := core.NewObjectFromMap(core.ValMap{"name": core.Str("a")}, ctx)
		set.Add(ctx, obj1)

		assert.True(t, bool(set.Has(ctx, obj1)))
		assert.False(t, bool(set.Has(ctx, obj2)))
	})

	//Tests with several transactions.

	t.Run("readonly transactions can read the Set in parallel", func(t *testing.T) {
		ctx1, ctx2, _ := sharedSetTestSetup2(t)
		defer ctx1.CancelGracefully()
		defer ctx2.CancelGracefully()

		readTx1 := core.StartNewReadonlyTransaction(ctx1)
		core.StartNewReadonlyTransaction(ctx2)

		pattern := NewSetPattern(SetConfig{
			Uniqueness: common.UniquenessConstraint{
				Type: common.UniqueRepr,
			},
		})

		set := NewSetWithConfig(ctx1, core.NewWrappedValueList(INT_1, INT_2), pattern.config)
		set.Share(ctx1.GetClosestState())

		assert.True(t, bool(set.Has(ctx1, INT_1)))
		assert.True(t, bool(set.Has(ctx2, INT_1)))

		assert.True(t, bool(set.Has(ctx1, INT_1)))
		assert.True(t, bool(set.Has(ctx2, INT_1)))

		assert.NoError(t, readTx1.Commit(ctx1))
		assert.True(t, bool(set.Has(ctx2, INT_1)))
	})
}

func TestSharedUnpersistedSetRemove(t *testing.T) {

	t.Run("remove an element of a URL-based uniqueness shared Set with no storage should cause a panic", func(t *testing.T) {
		ctx, _ := sharedSetTestSetup(t)
		defer ctx.CancelGracefully()

		pattern := NewSetPattern(SetConfig{
			Uniqueness: common.UniquenessConstraint{Type: common.UniqueURL},
		})

		set := NewSetWithConfig(ctx, nil, pattern.config)
		set.Share(ctx.GetClosestState())

		obj := core.NewObjectFromMap(core.ValMap{}, ctx)

		assert.PanicsWithValue(t, ErrURLUniquenessOnlySupportedIfPersistedSharedSet, func() {
			set.Remove(ctx, obj)
		})
	})

	t.Run("calling Remove with an element having the same property value as another element should have no impact", func(t *testing.T) {
		ctx, _ := sharedSetTestSetup(t)
		defer ctx.CancelGracefully()

		pattern := NewSetPattern(SetConfig{
			Uniqueness: common.UniquenessConstraint{
				Type:         common.UniquePropertyValue,
				PropertyName: "name",
			},
		})

		set := NewSetWithConfig(ctx, nil, pattern.config)
		set.Share(ctx.GetClosestState())

		obj1 := core.NewObjectFromMap(core.ValMap{"name": core.Str("a")}, ctx)
		obj2 := core.NewObjectFromMap(core.ValMap{"name": core.Str("a")}, ctx)

		set.Add(ctx, obj1)
		set.Remove(ctx, obj2)

		assert.True(t, bool(set.Has(ctx, obj1)))
		assert.False(t, bool(set.Has(ctx, obj2)))
	})
}
