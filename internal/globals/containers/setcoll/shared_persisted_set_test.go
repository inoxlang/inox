package setcoll

import (
	"path/filepath"
	"testing"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/permkind"
	"github.com/inoxlang/inox/internal/filekv"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/stretchr/testify/assert"

	"github.com/inoxlang/inox/internal/globals/containers/common"
)

const (
	MAX_MEM_FS_SIZE = 10_000
	INT_1           = core.Int(1)
	INT_2           = core.Int(2)

	INT_1_TYPED_REPR   = `{"int__value":1}`
	INT_2_TYPED_REPR   = `{"int__value":2}`
	INT_1_UNTYPED_REPR = `1`
	INT_2_UNTYPED_REPR = `2`
)

func TestSharedPersistedSetAdd(t *testing.T) {

	t.Run("url holder with no URL should get one", func(t *testing.T) {
		ctx, storage := sharedSetTestSetup(t)
		defer ctx.CancelGracefully()

		pattern := NewSetPattern(SetConfig{
			Uniqueness: common.UniquenessConstraint{
				Type: common.UniqueURL,
			},
		})

		storage.SetSerialized(ctx, "/set", `[]`)
		val, err := loadSet(ctx, core.FreeEntityLoadingParams{
			Key: "/set", Storage: storage, Pattern: pattern,
		})

		if !assert.NoError(t, err) {
			return
		}

		set := val.(*Set)
		set.Share(ctx.GetClosestState())

		obj := core.NewObjectFromMap(core.ValMap{}, ctx)
		set.Add(ctx, obj)

		url, ok := obj.URL()
		if !assert.True(t, ok) {
			return
		}

		assert.Regexp(t, "ldb://main/.*", string(url))
	})

	t.Run("adding an element of another URL-based container is not allowed", func(t *testing.T) {
		ctx, storage := sharedSetTestSetup(t)
		defer ctx.CancelGracefully()

		pattern := NewSetPattern(SetConfig{
			Uniqueness: common.UniquenessConstraint{
				Type: common.UniqueURL,
			},
		})

		storage.SetSerialized(ctx, "/set1", `[]`)
		storage.SetSerialized(ctx, "/set2", `[]`)

		val1, err := loadSet(ctx, core.FreeEntityLoadingParams{
			Key: "/set1", Storage: storage, Pattern: pattern,
		})

		if !assert.NoError(t, err) {
			return
		}

		val2, err := loadSet(ctx, core.FreeEntityLoadingParams{
			Key: "/set2", Storage: storage, Pattern: pattern,
		})

		if !assert.NoError(t, err) {
			return
		}

		set1 := val1.(*Set)
		set1.Share(ctx.GetClosestState())

		set2 := val2.(*Set)
		set2.Share(ctx.GetClosestState())

		obj := core.NewObjectFromMap(core.ValMap{}, ctx)
		set1.Add(ctx, obj)

		func() {
			defer func() {
				e := recover()
				if !assert.NotNil(t, e) {
					return
				}
				assert.ErrorIs(t, e.(error), common.ErrCannotAddURLToElemOfOtherContainer)
			}()

			set2.Add(ctx, obj)
		}()
	})

	t.Run("adding an element with the same property value as another element is not allowed", func(t *testing.T) {
		ctx, storage := sharedSetTestSetup(t)
		defer ctx.CancelGracefully()

		pattern := NewSetPattern(SetConfig{
			Element: core.NewInexactObjectPattern([]core.ObjectPatternEntry{{Name: "name", Pattern: core.STR_PATTERN}}),
			Uniqueness: common.UniquenessConstraint{
				Type:         common.UniquePropertyValue,
				PropertyName: "name",
			},
		})

		storage.SetSerialized(ctx, "/set1", `[]`)

		val1, err := loadSet(ctx, core.FreeEntityLoadingParams{
			Key: "/set1", Storage: storage, Pattern: pattern,
		})

		if !assert.NoError(t, err) {
			return
		}

		set := val1.(*Set)
		set.Share(ctx.GetClosestState())

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

	t.Run("Set should be persisted during call to .Add", func(t *testing.T) {
		ctx, storage := sharedSetTestSetup(t)
		defer ctx.CancelGracefully()

		defer ctx.CancelGracefully()

		pattern := NewSetPattern(SetConfig{
			Uniqueness: common.UniquenessConstraint{
				Type: common.UniqueRepr,
			},
		})

		storage.SetSerialized(ctx, "/set", `[]`)
		val, err := loadSet(ctx, core.FreeEntityLoadingParams{
			Key: "/set", Storage: storage, Pattern: pattern,
		})

		set := val.(*Set)

		set.Share(ctx.GetClosestState())

		if !assert.NoError(t, err) {
			return
		}

		set.Add(ctx, INT_1)

		//Check that the element is added from the tx's POV.

		assert.True(t, bool(set.Has(ctx, INT_1)))
		assert.True(t, bool(utils.Ret1(set.Get(ctx, core.String(INT_1_TYPED_REPR)))))
		values := core.IterateAllValuesOnly(ctx, set.Iterator(ctx, core.IteratorConfiguration{}))
		assert.ElementsMatch(t, []any{INT_1}, values)

		//Check that the Set is persisted

		persisted, err := loadSet(ctx, core.FreeEntityLoadingParams{
			Key: "/set", Storage: storage, Pattern: pattern,
		})

		if !assert.NoError(t, err) {
			return
		}

		assert.NotSame(t, persisted, set) //future-proofing the test

		vals := core.IterateAllValuesOnly(ctx, set.Iterator(ctx, core.IteratorConfiguration{}))
		assert.ElementsMatch(t, []any{INT_1}, vals)
	})

	t.Run("Set should be persisted at end of successful transaction if .Add was called transactionnaly", func(t *testing.T) {

		ctx, storage := sharedSetTestSetup(t)
		defer ctx.CancelGracefully()

		tx := core.StartNewTransaction(ctx)

		pattern := NewSetPattern(SetConfig{
			Uniqueness: common.UniquenessConstraint{
				Type: common.UniqueRepr,
			},
		})

		storage.SetSerialized(ctx, "/set", `[]`)
		val, err := loadSet(ctx, core.FreeEntityLoadingParams{
			Key: "/set", Storage: storage, Pattern: pattern,
		})

		set := val.(*Set)
		set.Share(ctx.GetClosestState())

		if !assert.NoError(t, err) {
			return
		}

		set.Add(ctx, INT_1)

		assert.True(t, bool(set.Has(ctx, INT_1)))
		values := core.IterateAllValuesOnly(ctx, set.Iterator(ctx, core.IteratorConfiguration{}))
		assert.ElementsMatch(t, []any{INT_1}, values)

		//Check that the Set is not persised

		persisted, err := loadSet(ctx, core.FreeEntityLoadingParams{
			Key: "/set", Storage: storage, Pattern: pattern,
		})

		if !assert.NoError(t, err) {
			return
		}

		assert.NotSame(t, persisted, set) //future-proofing the test
		assert.False(t, bool(persisted.(*Set).Has(ctx, INT_1)))

		assert.NoError(t, tx.Commit(ctx))

		//Check that the Set is persised

		persisted, err = loadSet(ctx, core.FreeEntityLoadingParams{
			Key: "/set", Storage: storage, Pattern: pattern,
		})

		if !assert.NoError(t, err) {
			return
		}

		assert.NotSame(t, persisted, set) //future-proofing the test
		assert.True(t, bool(persisted.(*Set).Has(ctx, INT_1)))
	})

	t.Run("Set should not be persisted at end of failed transaction if .Add was called transactionnaly", func(t *testing.T) {
		ctx1, ctx2, storage := sharedSetTestSetup2(t)
		defer ctx1.CancelGracefully()
		defer ctx2.CancelGracefully()

		pattern := NewSetPattern(SetConfig{
			Uniqueness: common.UniquenessConstraint{
				Type: common.UniqueRepr,
			},
		})

		storage.SetSerialized(ctx1, "/set", `[]`)
		val, err := loadSet(ctx1, core.FreeEntityLoadingParams{
			Key: "/set", Storage: storage, Pattern: pattern,
		})

		set := val.(*Set)
		set.Share(ctx1.GetClosestState())

		//The tx1 is started after the KV write in order
		//for the SetSerialized call to be already commited.
		tx1 := core.StartNewTransaction(ctx1)

		if !assert.NoError(t, err) {
			return
		}

		set.Add(ctx1, INT_1)

		//Check that the element is added from the tx's POV.

		assert.True(t, bool(set.Has(ctx1, INT_1)))
		assert.True(t, bool(set.Contains(ctx1, INT_1)))
		assert.True(t, bool(utils.Ret1(set.Get(ctx1, core.String(INT_1_TYPED_REPR)))))
		values := core.IterateAllValuesOnly(ctx1, set.Iterator(ctx1, core.IteratorConfiguration{}))
		assert.ElementsMatch(t, []any{INT_1}, values)

		//Check that the Set is not persised

		persisted, err := loadSet(ctx1, core.FreeEntityLoadingParams{
			Key: "/set", Storage: storage, Pattern: pattern,
		})

		if !assert.NoError(t, err) {
			return
		}

		assert.NotSame(t, persisted, set) //future-proofing the test
		assert.False(t, bool(persisted.(*Set).Has(ctx1, INT_1)))
		assert.False(t, bool(persisted.(*Set).Contains(ctx1, INT_1)))

		//roll back

		assert.NoError(t, tx1.Rollback(ctx1))

		//Check that the Set has not been updated from another transaction's POV.

		core.StartNewTransaction(ctx2)
		assert.False(t, bool(set.Has(ctx2, INT_1)))
		assert.False(t, bool(set.Contains(ctx2, INT_1)))

		//Check that the Set is not persised.

		persisted, err = loadSet(ctx1, core.FreeEntityLoadingParams{
			Key: "/set", Storage: storage, Pattern: pattern,
		})

		if !assert.NoError(t, err) {
			return
		}

		assert.NotSame(t, persisted, set) //future-proofing the test
		assert.False(t, bool(persisted.(*Set).Has(ctx1, INT_1)))
	})

	//Tests with several transactions.

	t.Run("a write transaction should wait for the previous write transaction to finish", func(t *testing.T) {
		ctx1, ctx2, storage := sharedSetTestSetup2(t)
		defer ctx1.CancelGracefully()
		defer ctx2.CancelGracefully()

		tx1 := core.StartNewTransaction(ctx1)
		core.StartNewTransaction(ctx2)

		pattern := NewSetPattern(SetConfig{
			Uniqueness: common.UniquenessConstraint{
				Type: common.UniqueRepr,
			},
		})

		storage.SetSerialized(ctx1, "/set", `[]`)
		val, err := loadSet(ctx1, core.FreeEntityLoadingParams{
			Key: "/set", Storage: storage, Pattern: pattern,
		})

		if !assert.NoError(t, err) {
			return
		}

		set := val.(*Set)
		set.Share(ctx1.GetClosestState())

		set.Add(ctx1, INT_1)
		assert.True(t, bool(set.Has(ctx1, INT_1)))

		tx2Done := make(chan struct{})
		go func() { //second transaction
			set.Add(ctx2, INT_2)

			//since the first transaction should be finished,
			//the other element should have been added.
			assert.True(t, bool(set.Has(ctx2, INT_1)))
			assert.True(t, bool(set.Has(ctx2, INT_2)))
			tx2Done <- struct{}{}
		}()

		assert.NoError(t, tx1.Commit(ctx1))

		<-tx2Done
	})

	t.Run("writes in two subsequent transactions", func(t *testing.T) {

		ctx1, ctx2, storage := sharedSetTestSetup2(t)
		defer ctx1.CancelGracefully()
		defer ctx2.CancelGracefully()

		tx1 := core.StartNewTransaction(ctx1)
		tx2 := core.StartNewTransaction(ctx2)

		pattern := NewSetPattern(SetConfig{
			Uniqueness: common.UniquenessConstraint{
				Type: common.UniqueRepr,
			},
		})

		storage.SetSerialized(ctx1, "/set", `[]`)
		val, err := loadSet(ctx1, core.FreeEntityLoadingParams{
			Key: "/set", Storage: storage, Pattern: pattern,
		})

		set := val.(*Set)

		//First transaction.

		set.Share(ctx1.GetClosestState())

		if !assert.NoError(t, err) {
			return
		}

		set.Add(ctx1, INT_1)
		if !assert.NoError(t, tx1.Commit(ctx1)) {
			return
		}

		//Second transaction.

		assert.True(t, bool(set.Has(ctx2, INT_1)))

		set.Add(ctx2, INT_2)
		assert.NoError(t, tx2.Commit(ctx2))

		//Check that the Set is persised

		persisted, err := loadSet(ctx2, core.FreeEntityLoadingParams{
			Key: "/set", Storage: storage, Pattern: pattern,
		})

		if !assert.NoError(t, err) {
			return
		}

		assert.NotSame(t, persisted, set) //future-proofing the test
		assert.True(t, bool(persisted.(*Set).Has(ctx2, INT_1)))
		assert.True(t, bool(persisted.(*Set).Has(ctx2, INT_2)))
	})

	// t.Run("if uniqueness is URL-based transactions should not wait for the previous transaction to finish", func(t *testing.T) {
	// 	ctx1, ctx2, storage := sharedSetTestSetup2(t)
	// 	defer ctx1.CancelGracefully()
	// 	defer ctx2.CancelGracefully()

	// 	tx1 := core.StartNewTransaction(ctx1)
	// 	core.StartNewTransaction(ctx2)

	// 	pattern := NewSetPattern(SetConfig{
	// 		Element: core.OBJECT_PATTERN,
	// 		Uniqueness: common.UniquenessConstraint{
	// 			Type: common.UniqueURL,
	// 		},
	// 	})

	// 	storage.SetSerialized(ctx1, "/set", `[]`)
	// 	val, err := loadSet(ctx1, core.FreeEntityLoadingParams{
	// 		Key: "/set", Storage: storage, Pattern: pattern,
	// 	})

	// 	if !assert.NoError(t, err) {
	// 		return
	// 	}

	// 	set := val.(*Set)
	// 	set.Share(ctx1.GetClosestState())

	// 	var (
	// 		OBJ_1 = core.NewObjectFromMapNoInit(core.ValMap{})
	// 		OBJ_2 = core.NewObjectFromMapNoInit(core.ValMap{})
	// 	)

	// 	set.Add(ctx1, OBJ_1)
	// 	//Check that 1 is added from all POVs (it's okay).
	// 	assert.True(t, bool(set.Has(ctx1, OBJ_1)))
	// 	assert.True(t, bool(set.Has(ctx2, OBJ_1)))

	// 	set.Add(ctx2, OBJ_2)
	// 	//Check that 2 is added from all POVs (it's okay).
	// 	assert.True(t, bool(set.Has(ctx2, OBJ_2)))
	// 	assert.True(t, bool(set.Has(ctx1, OBJ_2)))

	// 	assert.NoError(t, tx1.Commit(ctx1))

	// 	//Check that 1 is added from ctx2's POV.
	// 	assert.True(t, bool(set.Has(ctx2, OBJ_1)))

	// 	//Check that 2 is still added from ctx2's POV.
	// 	assert.True(t, bool(set.Has(ctx2, OBJ_2)))
	// })

}

func TestSharedPersistedSetRemove(t *testing.T) {

	t.Run("calling Remove with an element having the same property value as another element should have no impact", func(t *testing.T) {
		ctx, storage := sharedSetTestSetup(t)
		defer ctx.CancelGracefully()

		pattern := NewSetPattern(SetConfig{
			Element: core.NewInexactObjectPattern([]core.ObjectPatternEntry{{Name: "name", Pattern: core.STR_PATTERN}}),
			Uniqueness: common.UniquenessConstraint{
				Type:         common.UniquePropertyValue,
				PropertyName: "name",
			},
		})

		storage.SetSerialized(ctx, "/set1", `[]`)

		val1, err := loadSet(ctx, core.FreeEntityLoadingParams{
			Key: "/set1", Storage: storage, Pattern: pattern,
		})

		if !assert.NoError(t, err) {
			return
		}

		set := val1.(*Set)
		set.Share(ctx.GetClosestState())

		obj1 := core.NewObjectFromMap(core.ValMap{"name": core.String("a")}, ctx)
		obj2 := core.NewObjectFromMap(core.ValMap{"name": core.String("a")}, ctx)

		set.Add(ctx, obj1)
		set.Remove(ctx, obj2)

		assert.True(t, bool(set.Has(ctx, obj1)))
		assert.False(t, bool(set.Has(ctx, obj2)))
	})

	t.Run("Set should be persisted during call to .Remove", func(t *testing.T) {
		ctx, storage := sharedSetTestSetup(t)
		defer ctx.CancelGracefully()

		pattern := NewSetPattern(SetConfig{
			Uniqueness: common.UniquenessConstraint{
				Type: common.UniqueRepr,
			},
		})

		storage.SetSerialized(ctx, "/set", `[{"int__value":"1"}]`)
		val, err := loadSet(ctx, core.FreeEntityLoadingParams{
			Key: "/set", Storage: storage, Pattern: pattern,
		})

		if !assert.NoError(t, err) {
			return
		}

		set := val.(*Set)
		set.Share(ctx.GetClosestState())

		set.Remove(ctx, INT_1)

		//Check that the Set is persised

		persisted, err := loadSet(ctx, core.FreeEntityLoadingParams{
			Key: "/set", Storage: storage, Pattern: pattern,
		})

		if !assert.NoError(t, err) {
			return
		}

		assert.NotSame(t, persisted, set) //future-proofing the test

		vals := core.IterateAllValuesOnly(ctx, set.Iterator(ctx, core.IteratorConfiguration{}))
		assert.Len(t, vals, 0)
	})

	t.Run("Set should be persisted at end of successful transaction if .Remove was called transactionnaly", func(t *testing.T) {

		ctx, storage := sharedSetTestSetup(t)
		defer ctx.CancelGracefully()

		pattern := NewSetPattern(SetConfig{
			Uniqueness: common.UniquenessConstraint{
				Type: common.UniqueRepr,
			},
		})

		storage.SetSerialized(ctx, "/set", `[{"int__value":"1"}]`)
		val, err := loadSet(ctx, core.FreeEntityLoadingParams{
			Key: "/set", Storage: storage, Pattern: pattern,
		})

		tx := core.StartNewTransaction(ctx)

		if !assert.NoError(t, err) {
			return
		}

		set := val.(*Set)
		set.Share(ctx.GetClosestState())

		set.Remove(ctx, INT_1)

		//Check that the element is removed from the tx's POV.

		assert.False(t, bool(set.Has(ctx, INT_1)))
		assert.False(t, bool(utils.Ret1(set.Get(ctx, core.String(INT_1_TYPED_REPR)))))
		values := core.IterateAllValuesOnly(ctx, set.Iterator(ctx, core.IteratorConfiguration{}))
		assert.Empty(t, values)

		//Check that the Set is not persised yet.

		persisted, err := loadSet(ctx, core.FreeEntityLoadingParams{
			Key: "/set", Storage: storage, Pattern: pattern,
		})

		if !assert.NoError(t, err) {
			return
		}

		assert.NotSame(t, persisted, set) //future-proofing the test
		assert.True(t, bool(persisted.(*Set).Has(ctx, INT_1)))

		assert.NoError(t, tx.Commit(ctx))

		//Check that the Set is not persised

		persisted, err = loadSet(ctx, core.FreeEntityLoadingParams{
			Key: "/set", Storage: storage, Pattern: pattern,
		})

		if !assert.NoError(t, err) {
			return
		}

		assert.NotSame(t, persisted, set) //future-proofing the test
		assert.False(t, bool(persisted.(*Set).Has(ctx, INT_1)))
	})

	t.Run("Set should not be persisted at end of failed transaction if .Remove was called transactionnaly", func(t *testing.T) {
		ctx1, ctx2, storage := sharedSetTestSetup2(t)
		defer ctx1.CancelGracefully()
		defer ctx2.CancelGracefully()

		pattern := NewSetPattern(SetConfig{
			Uniqueness: common.UniquenessConstraint{
				Type: common.UniqueRepr,
			},
		})

		storage.SetSerialized(ctx1, "/set", `[{"int__value":1}]`)
		val, err := loadSet(ctx1, core.FreeEntityLoadingParams{
			Key: "/set", Storage: storage, Pattern: pattern,
		})

		set := val.(*Set)
		set.Share(ctx1.GetClosestState())

		tx1 := core.StartNewTransaction(ctx1)

		if !assert.NoError(t, err) {
			return
		}

		if !assert.True(t, bool(set.Has(ctx1, INT_1))) {
			return
		}

		set.Remove(ctx1, INT_1)

		//Check that the Set is not persised

		persisted, err := loadSet(ctx1, core.FreeEntityLoadingParams{
			Key: "/set", Storage: storage, Pattern: pattern,
		})

		if !assert.NoError(t, err) {
			return
		}

		assert.NotSame(t, persisted, set) //future-proofing the test
		assert.True(t, bool(persisted.(*Set).Has(ctx1, INT_1)))

		//roll back

		assert.NoError(t, tx1.Rollback(ctx1))

		//Check that the Set has not been updated from another transaction's POV.

		core.StartNewTransaction(ctx2)
		assert.True(t, bool(set.Has(ctx2, INT_1)))
		assert.True(t, bool(set.Contains(ctx2, INT_1)))

		//Check that the Set is not persised

		persisted, err = loadSet(ctx1, core.FreeEntityLoadingParams{
			Key: "/set", Storage: storage, Pattern: pattern,
		})

		if !assert.NoError(t, err) {
			return
		}

		assert.NotSame(t, persisted, set) //future-proofing the test
		assert.True(t, bool(persisted.(*Set).Has(ctx1, INT_1)))
	})

	//Testswith several transactions.

	t.Run("if uniqueness is not URL-based, write transactions should wait for the previous transaction to finish", func(t *testing.T) {
		ctx1, ctx2, storage := sharedSetTestSetup2(t)
		defer ctx1.CancelGracefully()
		defer ctx2.CancelGracefully()

		tx1 := core.StartNewTransaction(ctx1)
		core.StartNewTransaction(ctx2)

		pattern := NewSetPattern(SetConfig{
			Uniqueness: common.UniquenessConstraint{
				Type: common.UniqueRepr,
			},
		})

		storage.SetSerialized(ctx1, "/set", `[1, 2]`)
		val, err := loadSet(ctx1, core.FreeEntityLoadingParams{
			Key: "/set", Storage: storage, Pattern: pattern,
		})

		if !assert.NoError(t, err) {
			return
		}

		set := val.(*Set)
		set.Share(ctx1.GetClosestState())

		set.Remove(ctx1, INT_1)
		assert.False(t, bool(set.Has(ctx1, INT_1)))

		tx2Done := make(chan struct{})
		go func() { //second transaction
			set.Remove(ctx2, INT_2)

			//since the first transaction should be finished,
			//the other element should have been removed.
			assert.False(t, bool(set.Has(ctx2, INT_1)))
			assert.False(t, bool(set.Has(ctx2, INT_2)))
			tx2Done <- struct{}{}
		}()

		assert.NoError(t, tx1.Commit(ctx1))
		<-tx2Done
	})

}

func TestSharedPersistedSetHas(t *testing.T) {

	t.Run("an element with the same property value as another element is not considered to be in the set", func(t *testing.T) {
		ctx, storage := sharedSetTestSetup(t)
		defer ctx.CancelGracefully()

		pattern := NewSetPattern(SetConfig{
			Element: core.NewInexactObjectPattern([]core.ObjectPatternEntry{{Name: "name", Pattern: core.STR_PATTERN}}),
			Uniqueness: common.UniquenessConstraint{
				Type:         common.UniquePropertyValue,
				PropertyName: "name",
			},
		})

		storage.SetSerialized(ctx, "/set1", `[]`)

		val1, err := loadSet(ctx, core.FreeEntityLoadingParams{
			Key: "/set1", Storage: storage, Pattern: pattern,
		})

		if !assert.NoError(t, err) {
			return
		}

		set := val1.(*Set)
		set.Share(ctx.GetClosestState())

		obj1 := core.NewObjectFromMap(core.ValMap{"name": core.String("a")}, ctx)
		obj2 := core.NewObjectFromMap(core.ValMap{"name": core.String("a")}, ctx)
		set.Add(ctx, obj1)

		assert.True(t, bool(set.Has(ctx, obj1)))
		assert.False(t, bool(set.Has(ctx, obj2)))
	})

	//Tests with several transactions.

	t.Run("readonly transactions can read the Set in parallel", func(t *testing.T) {
		ctx1, ctx2, storage := sharedSetTestSetup2(t)
		defer ctx1.CancelGracefully()
		defer ctx2.CancelGracefully()

		readTx1 := core.StartNewReadonlyTransaction(ctx1)
		core.StartNewReadonlyTransaction(ctx2)

		pattern := NewSetPattern(SetConfig{
			Uniqueness: common.UniquenessConstraint{
				Type: common.UniqueRepr,
			},
		})

		storage.SetSerialized(ctx1, "/set", `[{"int__value":1}]`)
		val, err := loadSet(ctx1, core.FreeEntityLoadingParams{
			Key: "/set", Storage: storage, Pattern: pattern,
		})

		if !assert.NoError(t, err) {
			return
		}

		set := val.(*Set)
		set.Share(ctx1.GetClosestState())

		assert.True(t, bool(set.Has(ctx1, INT_1)))
		assert.True(t, bool(set.Has(ctx2, INT_1)))

		assert.True(t, bool(set.Has(ctx1, INT_1)))
		assert.True(t, bool(set.Has(ctx2, INT_1)))

		assert.NoError(t, readTx1.Commit(ctx1))
		assert.True(t, bool(set.Has(ctx2, INT_1)))
	})
}

func TestInteractWithElementsOfLoadedSet(t *testing.T) {

	t.Run("adding a simple value property to an element should trigger a persistence", func(t *testing.T) {
		ctx, storage := sharedSetTestSetup(t)
		defer ctx.CancelGracefully()

		pattern := NewSetPattern(SetConfig{
			Uniqueness: common.UniquenessConstraint{
				Type: common.UniqueURL,
			},
		})

		storage.SetSerialized(ctx, "/set", `[]`)
		set, err := loadSet(ctx, core.FreeEntityLoadingParams{
			Key: "/set", Storage: storage, Pattern: pattern,
		})

		if !assert.NoError(t, err) {
			return
		}

		newElem := core.NewObjectFromMap(core.ValMap{}, ctx)
		set.(*Set).Add(ctx, newElem)

		url, _ := newElem.URL()

		//load again

		loadedSet, err := loadSet(ctx, core.FreeEntityLoadingParams{
			Key: "/set", Storage: storage, Pattern: pattern,
		})

		if !assert.NoError(t, err) {
			return
		}

		assert.NotSame(t, set, loadedSet) //future-proofing the test

		elem, _ := loadedSet.(*Set).Get(ctx, core.String(url.GetLastPathSegment()))
		obj := elem.(*core.Object)
		if !assert.NoError(t, obj.SetProp(ctx, "prop", INT_1)) {
			return
		}

		//load again

		loadedSet, err = loadSet(ctx, core.FreeEntityLoadingParams{
			Key: "/set", Storage: storage, Pattern: pattern,
		})
		if !assert.NoError(t, err) {
			return
		}

		loadedElem, _ := loadedSet.(*Set).Get(ctx, core.String(url.GetLastPathSegment()))
		loadedObj := loadedElem.(*core.Object)

		if !assert.Equal(t, []string{"prop"}, loadedObj.PropertyNames(ctx)) {
			return
		}

		assert.Equal(t, INT_1, loadedObj.Prop(ctx, "prop"))
	})
}

func sharedSetTestSetup(t *testing.T) (*core.Context, core.DataStore) {
	ctx := core.NewContextWithEmptyState(core.ContextConfig{
		Permissions: []core.Permission{
			core.DatabasePermission{
				Kind_:  permkind.Read,
				Entity: core.Host("ldb://main"),
			},
			core.DatabasePermission{
				Kind_:  permkind.Write,
				Entity: core.Host("ldb://main"),
			},
		},
	}, nil)
	kv := utils.Must(filekv.OpenSingleFileKV(filekv.KvStoreConfig{
		Path: core.PathFrom(filepath.Join(t.TempDir(), "kv")),
	}))
	storage := filekv.NewSerializedValueStorage(kv, "ldb://main/")
	return ctx, storage
}

func sharedSetTestSetup2(t *testing.T) (*core.Context, *core.Context, core.DataStore) {
	config := core.ContextConfig{
		Permissions: []core.Permission{
			core.DatabasePermission{
				Kind_:  permkind.Read,
				Entity: core.Host("ldb://main"),
			},
			core.DatabasePermission{
				Kind_:  permkind.Write,
				Entity: core.Host("ldb://main"),
			},
		},
	}

	ctx1 := core.NewContextWithEmptyState(config, nil)
	ctx2 := core.NewContextWithEmptyState(config, nil)

	kv := utils.Must(filekv.OpenSingleFileKV(filekv.KvStoreConfig{
		Path: core.PathFrom(filepath.Join(t.TempDir(), "kv")),
	}))
	storage := filekv.NewSerializedValueStorage(kv, "ldb://main/")
	return ctx1, ctx2, storage
}
