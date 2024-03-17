package setcoll

import (
	"testing"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/utils"
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
		set.Share(ctx.MustGetClosestState())

		set.Add(ctx, INT_1)
		assert.True(t, bool(set.Has(ctx, INT_1)))

		assert.NoError(t, tx.Commit(ctx))

		otherCtx, _ := sharedSetTestSetup(t)
		defer ctx.CancelGracefully()

		assert.True(t, bool(set.Has(otherCtx, INT_1)))
	})

	t.Run("adding an element to a URL-based uniqueness shared Set with no storage should cause a panic", func(t *testing.T) {
		ctx, _ := sharedSetTestSetup(t)
		defer ctx.CancelGracefully()

		pattern := NewSetPattern(SetConfig{
			Uniqueness: common.UniquenessConstraint{Type: common.UniqueURL},
		})

		set := NewSetWithConfig(ctx, nil, pattern.config)
		set.Share(ctx.MustGetClosestState())

		obj := core.NewObjectFromMap(core.ValMap{}, ctx)

		assert.PanicsWithValue(t, ErrURLUniquenessOnlySupportedIfPersistedSharedSet, func() {
			set.Add(ctx, obj)
		})
	})

	//Tests with several transactions.

	t.Run("transactions should wait for the previous transaction to finish", func(t *testing.T) {
		ctx1 := core.NewContextWithEmptyState(core.ContextConfig{}, nil)
		defer ctx1.CancelGracefully()
		tx1 := core.StartNewTransaction(ctx1)

		ctx2 := core.NewContextWithEmptyState(core.ContextConfig{}, nil)
		defer ctx2.CancelGracefully()
		core.StartNewTransaction(ctx2)

		set := NewSetWithConfig(ctx1, nil, SetConfig{
			Element: core.INT_PATTERN,
			Uniqueness: common.UniquenessConstraint{
				Type: common.UniqueRepr,
			},
		})
		set.Share(ctx1.MustGetClosestState())

		set.Add(ctx1, INT_1)

		//Check that the element is added from tx1's POV.
		assert.True(t, bool(set.Has(ctx1, INT_1)))
		assert.True(t, bool(utils.Ret1(set.Get(ctx1, core.String(INT_1_UNTYPED_REPR)))))
		values := core.IterateAllValuesOnly(ctx1, set.Iterator(ctx1, core.IteratorConfiguration{}))
		assert.ElementsMatch(t, []any{INT_1}, values)

		tx2Done := make(chan struct{})
		go func() { //second transaction
			set.Add(ctx2, INT_2)

			//since the first transaction should be finished,
			//the other element should have been added.
			assert.True(t, bool(set.Has(ctx2, INT_1)))
			assert.True(t, bool(utils.Ret1(set.Get(ctx2, core.String(INT_1_UNTYPED_REPR)))))

			assert.True(t, bool(set.Has(ctx2, INT_2)))
			assert.True(t, bool(utils.Ret1(set.Get(ctx2, core.String(INT_2_UNTYPED_REPR)))))

			values := core.IterateAllValuesOnly(ctx2, set.Iterator(ctx2, core.IteratorConfiguration{}))
			assert.ElementsMatch(t, []any{INT_1, INT_2}, values)

			tx2Done <- struct{}{}
		}()

		assert.NoError(t, tx1.Commit(ctx1))

		<-tx2Done
	})

	t.Run("writes in subsequent transactions", func(t *testing.T) {
		ctx1 := core.NewContextWithEmptyState(core.ContextConfig{}, nil)
		defer ctx1.CancelGracefully()
		tx1 := core.StartNewTransaction(ctx1)

		ctx2 := core.NewContextWithEmptyState(core.ContextConfig{}, nil)
		defer ctx2.CancelGracefully()
		tx2 := core.StartNewTransaction(ctx2)

		ctx3 := core.NewContextWithEmptyState(core.ContextConfig{}, nil)
		defer ctx3.CancelGracefully()
		core.StartNewTransaction(ctx3)

		set := NewSetWithConfig(ctx1, nil, SetConfig{
			Uniqueness: common.UniquenessConstraint{
				Type: common.UniqueRepr,
			},
		})
		set.Share(ctx1.MustGetClosestState())

		//First transaction.

		set.Add(ctx1, INT_1)
		if !assert.NoError(t, tx1.Commit(ctx1)) {
			return
		}

		//Second transaction.

		assert.True(t, bool(set.Has(ctx2, INT_1)))

		set.Add(ctx2, INT_2)
		if !assert.NoError(t, tx2.Commit(ctx2)) {
			return
		}

		//Third transaction.
		assert.True(t, bool(set.Has(ctx3, INT_1)))
		assert.True(t, bool(set.Has(ctx3, INT_2)))
	})

	t.Run("adding an element with the same property value as another element is not allowed", func(t *testing.T) {
		ctx, _ := sharedSetTestSetup(t)
		defer ctx.CancelGracefully()

		pattern := NewSetPattern(SetConfig{
			Element: core.NewInexactObjectPattern([]core.ObjectPatternEntry{{Name: "name", Pattern: core.STR_PATTERN}}),
			Uniqueness: common.UniquenessConstraint{
				Type:         common.UniquePropertyValue,
				PropertyName: "name",
			},
		})

		set := NewSetWithConfig(ctx, nil, pattern.config)
		set.Share(ctx.MustGetClosestState())

		obj1 := core.NewObjectFromMap(core.ValMap{"name": core.String("a")}, ctx)
		obj2 := core.NewObjectFromMap(core.ValMap{"name": core.String("a")}, ctx)
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
		set.Share(ctx.MustGetClosestState())

		obj := core.NewObjectFromMap(core.ValMap{}, ctx)

		assert.PanicsWithValue(t, ErrURLUniquenessOnlySupportedIfPersistedSharedSet, func() {
			set.Has(ctx, obj)
		})
	})

	t.Run("an element with the same property value as another element is not considered to be in the set", func(t *testing.T) {
		ctx, _ := sharedSetTestSetup(t)
		defer ctx.CancelGracefully()

		pattern := NewSetPattern(SetConfig{
			Element: core.NewInexactObjectPattern([]core.ObjectPatternEntry{{Name: "name", Pattern: core.STR_PATTERN}}),
			Uniqueness: common.UniquenessConstraint{
				Type:         common.UniquePropertyValue,
				PropertyName: "name",
			},
		})

		set := NewSetWithConfig(ctx, nil, pattern.config)
		set.Share(ctx.MustGetClosestState())

		obj1 := core.NewObjectFromMap(core.ValMap{"name": core.String("a")}, ctx)
		obj2 := core.NewObjectFromMap(core.ValMap{"name": core.String("a")}, ctx)
		set.Add(ctx, obj1)

		assert.True(t, bool(set.Has(ctx, obj1)))
		assert.False(t, bool(set.Has(ctx, obj2)))
	})

	t.Run("Has should be thread safe", func(t *testing.T) {
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

		set.Share(ctx1.MustGetClosestState())

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
				callCount++
				set.Has(ctx1, INT_1)
			}
		}

		assert.Greater(t, callCount, ADD_COUNT/100) //just make sure the function was called several times.
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
		set.Share(ctx1.MustGetClosestState())

		assert.True(t, bool(set.Has(ctx1, INT_1)))
		assert.True(t, bool(set.Has(ctx2, INT_1)))

		assert.True(t, bool(set.Has(ctx1, INT_2)))
		assert.True(t, bool(set.Has(ctx2, INT_2)))

		assert.NoError(t, readTx1.Commit(ctx1))
		assert.True(t, bool(set.Has(ctx2, INT_1)))
	})
}

func TestSharedUnpersistedSetContains(t *testing.T) {

	t.Run("checking the existence of an element of a URL-based uniqueness shared Set with no storage should cause a panic", func(t *testing.T) {
		ctx, _ := sharedSetTestSetup(t)
		defer ctx.CancelGracefully()

		pattern := NewSetPattern(SetConfig{
			Uniqueness: common.UniquenessConstraint{Type: common.UniqueURL},
		})

		set := NewSetWithConfig(ctx, nil, pattern.config)
		set.Share(ctx.MustGetClosestState())

		obj := core.NewObjectFromMap(core.ValMap{}, ctx)

		assert.PanicsWithValue(t, ErrURLUniquenessOnlySupportedIfPersistedSharedSet, func() {
			set.Contains(ctx, obj)
		})
	})

	t.Run("an element with the same property value as another element is not considered to be in the set", func(t *testing.T) {
		ctx, _ := sharedSetTestSetup(t)
		defer ctx.CancelGracefully()

		pattern := NewSetPattern(SetConfig{
			Element: core.NewInexactObjectPattern([]core.ObjectPatternEntry{{Name: "name", Pattern: core.STRING_PATTERN}}),
			Uniqueness: common.UniquenessConstraint{
				Type:         common.UniquePropertyValue,
				PropertyName: "name",
			},
		})

		set := NewSetWithConfig(ctx, nil, pattern.config)
		set.Share(ctx.MustGetClosestState())

		obj1 := core.NewObjectFromMap(core.ValMap{"name": core.String("a")}, ctx)
		obj2 := core.NewObjectFromMap(core.ValMap{"name": core.String("a")}, ctx)
		set.Add(ctx, obj1)

		assert.True(t, bool(set.Contains(ctx, obj1)))
		assert.False(t, bool(set.Contains(ctx, obj2)))
	})

	t.Run("Contains should be thread safe", func(t *testing.T) {
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

		set.Share(ctx1.MustGetClosestState())

		done := make(chan struct{})
		go func() {
			for i := 0; i < 100_000; i++ {
				set.Add(ctx2, core.Int(i+5))
			}
			done <- struct{}{}
		}()

		for i := 0; i < 100_000; i++ {
			set.Contains(ctx1, INT_1)
		}

		<-done
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
		set.Share(ctx1.MustGetClosestState())

		assert.True(t, bool(set.Contains(ctx1, INT_1)))
		assert.True(t, bool(set.Contains(ctx2, INT_1)))

		assert.True(t, bool(set.Contains(ctx1, INT_1)))
		assert.True(t, bool(set.Contains(ctx2, INT_1)))

		assert.NoError(t, readTx1.Commit(ctx1))
		assert.True(t, bool(set.Contains(ctx2, INT_1)))
	})
}

func TestSharedUnpersistedSetGetElementByKey(t *testing.T) {

	t.Run("GetElementByKey should be thread safe", func(t *testing.T) {
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

		set.Share(ctx1.MustGetClosestState())

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
				callCount++
				set.GetElementByKey(ctx1, INT_1_TYPED_REPR)
			}
		}

		assert.Greater(t, callCount, ADD_COUNT/100) //just make sure the function was called several times.
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
		set.Share(ctx1.MustGetClosestState())

		//Check that INT_1 is in the Set.

		elemKey1 := set.getElementPathKeyFromKey(INT_1_TYPED_REPR)
		elemKey2 := set.getElementPathKeyFromKey(INT_2_TYPED_REPR)

		elem, err := set.GetElementByKey(ctx1, elemKey1)
		if !assert.NoError(t, err) {
			return
		}
		assert.Equal(t, INT_1, elem)

		elem, err = set.GetElementByKey(ctx2, elemKey1)
		if !assert.NoError(t, err) {
			return
		}
		assert.Equal(t, INT_1, elem)

		//Check that INT_2 is in the Set.

		elem, err = set.GetElementByKey(ctx1, elemKey2)
		if !assert.NoError(t, err) {
			return
		}
		assert.Equal(t, INT_2, elem)

		elem, err = set.GetElementByKey(ctx2, elemKey2)
		if !assert.NoError(t, err) {
			return
		}
		assert.Equal(t, INT_2, elem)

		//Commit the first transaction.
		assert.NoError(t, readTx1.Commit(ctx1))

		elem, err = set.GetElementByKey(ctx2, elemKey1)
		if !assert.NoError(t, err) {
			return
		}
		assert.Equal(t, INT_1, elem)
	})
}

func TestSharedUnpersistedSetGet(t *testing.T) {

	t.Run("Get should be thread safe", func(t *testing.T) {
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

		set.Share(ctx1.MustGetClosestState())

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
				callCount++
				set.Get(ctx1, core.String(INT_1_TYPED_REPR))
			}
		}

		assert.Greater(t, callCount, ADD_COUNT/100) //just make sure the function was called several times.
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
		set.Share(ctx1.MustGetClosestState())

		//Check that INT_1 is in the Set.

		elem, ok := set.Get(ctx1, core.String(INT_1_TYPED_REPR))
		if !assert.True(t, bool(ok)) {
			return
		}
		assert.Equal(t, INT_1, elem)

		elem, ok = set.Get(ctx2, core.String(INT_1_TYPED_REPR))
		if !assert.True(t, bool(ok)) {
			return
		}
		assert.Equal(t, INT_1, elem)

		//Check that INT_2 is in the Set.

		elem, ok = set.Get(ctx1, core.String(INT_2_TYPED_REPR))
		if !assert.True(t, bool(ok)) {
			return
		}
		assert.Equal(t, INT_2, elem)

		elem, ok = set.Get(ctx2, core.String(INT_2_TYPED_REPR))
		if !assert.True(t, bool(ok)) {
			return
		}
		assert.Equal(t, INT_2, elem)

		//Commit the first transaction.
		assert.NoError(t, readTx1.Commit(ctx1))

		elem, ok = set.Get(ctx2, core.String(INT_1_TYPED_REPR))
		if !assert.True(t, bool(ok)) {
			return
		}
		assert.Equal(t, INT_1, elem)
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
		set.Share(ctx.MustGetClosestState())

		obj := core.NewObjectFromMap(core.ValMap{}, ctx)

		assert.PanicsWithValue(t, ErrURLUniquenessOnlySupportedIfPersistedSharedSet, func() {
			set.Remove(ctx, obj)
		})
	})

	t.Run("calling Remove with an element having the same property value as another element should have no impact", func(t *testing.T) {
		ctx, _ := sharedSetTestSetup(t)
		defer ctx.CancelGracefully()

		pattern := NewSetPattern(SetConfig{
			Element: core.NewInexactObjectPattern([]core.ObjectPatternEntry{{Name: "name", Pattern: core.STRING_PATTERN}}),
			Uniqueness: common.UniquenessConstraint{
				Type:         common.UniquePropertyValue,
				PropertyName: "name",
			},
		})

		set := NewSetWithConfig(ctx, nil, pattern.config)
		set.Share(ctx.MustGetClosestState())

		obj1 := core.NewObjectFromMap(core.ValMap{"name": core.String("a")}, ctx)
		obj2 := core.NewObjectFromMap(core.ValMap{"name": core.String("a")}, ctx)

		set.Add(ctx, obj1)
		set.Remove(ctx, obj2)

		assert.True(t, bool(set.Has(ctx, obj1)))
		assert.False(t, bool(set.Has(ctx, obj2)))

		values := core.IterateAllValuesOnly(ctx, set.Iterator(ctx, core.IteratorConfiguration{}))
		assert.ElementsMatch(t, []any{obj1}, values)
	})
}
