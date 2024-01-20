package setcoll

import (
	"testing"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/stretchr/testify/assert"

	"github.com/inoxlang/inox/internal/globals/containers/common"
)

func TestSharedUnpersistedSetAdd(t *testing.T) {
	t.Run("transient Set should be updated if .Add was called transactionnaly", func(t *testing.T) {
		int1 := core.Int(1)

		ctx1, _ := sharedSetTestSetup(t)
		tx1 := core.StartNewTransaction(ctx1)
		defer ctx1.CancelGracefully()

		ctx2 := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
		core.StartNewTransaction(ctx2)
		defer ctx2.CancelGracefully()

		pattern := NewSetPattern(SetConfig{
			Uniqueness: common.UniquenessConstraint{Type: common.UniqueRepr},
		})

		set := NewSetWithConfig(ctx1, nil, pattern.config)
		set.Share(ctx1.GetClosestState())

		set.Add(ctx1, int1)

		//check that the Set is not updated from the other ctx2's POV
		assert.False(t, bool(set.Has(ctx2, int1)))

		//commit the transaction associated with ctx1
		utils.PanicIfErr(tx1.Commit(ctx1))

		//check that the Set is updated from the other ctx's POV
		assert.True(t, bool(set.Has(ctx2, int1)))
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
}
