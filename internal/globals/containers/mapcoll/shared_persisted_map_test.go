package mapcoll

import (
	"testing"

	"github.com/inoxlang/inox/internal/core"
	"github.com/stretchr/testify/assert"
)

const (
	MAX_MEM_FS_SIZE = 10_000
)

func TestSharedPersistedMapSet(t *testing.T) {

	t.Run("Set should be persisted during call to .Set", func(t *testing.T) {
		ctx, storage := sharedSetTestSetup(t)
		defer ctx.CancelGracefully()

		pattern := NewMapPattern(MapConfig{})

		storage.SetSerialized(ctx, "/map", `[]`)
		val, err := loadMap(ctx, core.FreeEntityLoadingParams{
			Key: "/map", Storage: storage, Pattern: pattern,
		})

		m := val.(*Map)

		m.Share(ctx.GetClosestState())

		if !assert.NoError(t, err) {
			return
		}

		m.Set(ctx, INT_1, STRING_A)

		//Check that entry is added.

		assert.True(t, bool(m.Has(ctx, INT_1)))
		entryValue, ok := m.Get(ctx, INT_1)
		if assert.True(t, bool(ok)) {
			assert.Equal(t, STRING_A, entryValue)
		}

		values := core.IterateAllValuesOnly(ctx, m.Iterator(ctx, core.IteratorConfiguration{}))
		assert.ElementsMatch(t, []any{STRING_A}, values)

		//Check that the Map is persisted.

		persisted, err := loadMap(ctx, core.FreeEntityLoadingParams{
			Key: "/map", Storage: storage, Pattern: pattern,
		})

		if !assert.NoError(t, err) {
			return
		}

		assert.NotSame(t, persisted, m) //future-proofing the test

		values = core.IterateAllValuesOnly(ctx, m.Iterator(ctx, core.IteratorConfiguration{}))
		assert.ElementsMatch(t, []any{STRING_A}, values)
	})

	t.Run("Set should be persisted at end of successful transaction if .Add was called transactionnaly", func(t *testing.T) {

		ctx, storage := sharedSetTestSetup(t)
		defer ctx.CancelGracefully()

		tx := core.StartNewTransaction(ctx)

		pattern := NewMapPattern(MapConfig{})

		storage.SetSerialized(ctx, "/map", `[]`)
		val, err := loadMap(ctx, core.FreeEntityLoadingParams{
			Key: "/map", Storage: storage, Pattern: pattern,
		})

		m := val.(*Map)
		m.Share(ctx.GetClosestState())

		if !assert.NoError(t, err) {
			return
		}

		m.Set(ctx, INT_1, STRING_A)

		//Check that entry is added from the tx's POV.

		assert.True(t, bool(m.Has(ctx, INT_1)))
		entryValue, ok := m.Get(ctx, INT_1)
		if assert.True(t, bool(ok)) {
			assert.Equal(t, STRING_A, entryValue)
		}

		values := core.IterateAllValuesOnly(ctx, m.Iterator(ctx, core.IteratorConfiguration{}))
		assert.ElementsMatch(t, []any{STRING_A}, values)

		//Check that the Map is not persised.

		persisted, err := loadMap(ctx, core.FreeEntityLoadingParams{
			Key: "/map", Storage: storage, Pattern: pattern,
		})

		if !assert.NoError(t, err) {
			return
		}

		assert.NotSame(t, persisted, m) //future-proofing the test
		assert.False(t, bool(persisted.(*Map).Has(ctx, INT_1)))

		assert.NoError(t, tx.Commit(ctx))

		//Check that the Map is persised

		persisted, err = loadMap(ctx, core.FreeEntityLoadingParams{
			Key: "/map", Storage: storage, Pattern: pattern,
		})

		if !assert.NoError(t, err) {
			return
		}

		assert.NotSame(t, persisted, m) //future-proofing the test
		assert.True(t, bool(persisted.(*Map).Has(ctx, INT_1)))
		assert.True(t, bool(persisted.(*Map).Contains(ctx, STRING_A)))
	})

	t.Run("Set should not be persisted at end of failed transaction if .Add was called transactionnaly", func(t *testing.T) {
		ctx1, ctx2, storage := sharedSetTestSetup2(t)
		defer ctx1.CancelGracefully()
		defer ctx2.CancelGracefully()

		pattern := NewMapPattern(MapConfig{})

		storage.SetSerialized(ctx1, "/map", `[]`)
		val, err := loadMap(ctx1, core.FreeEntityLoadingParams{
			Key: "/map", Storage: storage, Pattern: pattern,
		})

		m := val.(*Map)

		m.Share(ctx1.GetClosestState())

		//The tx is started after the KV write in order
		//for the write to be already commited.
		tx := core.StartNewTransaction(ctx1)

		if !assert.NoError(t, err) {
			return
		}

		m.Set(ctx1, INT_1, STRING_A)

		//Check that entry is added.

		assert.True(t, bool(m.Has(ctx1, INT_1)))
		entryValue, ok := m.Get(ctx1, INT_1)
		if assert.True(t, bool(ok)) {
			assert.Equal(t, STRING_A, entryValue)
		}

		values := core.IterateAllValuesOnly(ctx1, m.Iterator(ctx1, core.IteratorConfiguration{}))
		assert.ElementsMatch(t, []any{STRING_A}, values)

		//Check that the Map is not persised.

		persisted, err := loadMap(ctx1, core.FreeEntityLoadingParams{
			Key: "/map", Storage: storage, Pattern: pattern,
		})

		if !assert.NoError(t, err) {
			return
		}

		assert.NotSame(t, persisted, m) //future-proofing the test
		assert.False(t, bool(persisted.(*Map).Has(ctx1, INT_1)))
		assert.False(t, bool(persisted.(*Map).Contains(ctx1, STRING_A)))

		//roll back

		assert.NoError(t, tx.Rollback(ctx1))

		//Check that the Map has not been updated from another transaction's POV.

		core.StartNewTransaction(ctx2)
		assert.False(t, bool(m.Has(ctx2, INT_1)))
		assert.False(t, bool(m.Contains(ctx2, INT_1)))

		//Check that the Map is not persised.

		persisted, err = loadMap(ctx1, core.FreeEntityLoadingParams{
			Key: "/map", Storage: storage, Pattern: pattern,
		})

		if !assert.NoError(t, err) {
			return
		}

		assert.NotSame(t, persisted, m) //future-proofing the test
		assert.False(t, bool(persisted.(*Map).Has(ctx1, INT_1)))
		assert.False(t, bool(persisted.(*Map).Contains(ctx1, STRING_A)))
	})

	//Tests with several transactions.

	t.Run("a write transaction should wait for the previous write transaction to finish", func(t *testing.T) {
		ctx1, ctx2, storage := sharedSetTestSetup2(t)
		defer ctx1.CancelGracefully()
		defer ctx2.CancelGracefully()

		tx1 := core.StartNewTransaction(ctx1)
		core.StartNewTransaction(ctx2)

		pattern := NewMapPattern(MapConfig{})

		storage.SetSerialized(ctx1, "/map", `[]`)
		val, err := loadMap(ctx1, core.FreeEntityLoadingParams{
			Key: "/map", Storage: storage, Pattern: pattern,
		})

		if !assert.NoError(t, err) {
			return
		}

		m := val.(*Map)
		m.Share(ctx1.GetClosestState())

		m.Set(ctx1, INT_1, STRING_A)

		//Check that entry is added from tx1's POV.

		assert.True(t, bool(m.Has(ctx1, INT_1)))
		entryValue, ok := m.Get(ctx1, INT_1)
		if assert.True(t, bool(ok)) {
			assert.Equal(t, STRING_A, entryValue)
		}

		values := core.IterateAllValuesOnly(ctx1, m.Iterator(ctx1, core.IteratorConfiguration{}))
		assert.ElementsMatch(t, []any{STRING_A}, values)

		tx2Done := make(chan struct{})
		go func() { //second transaction
			m.Set(ctx2, INT_2, STRING_B)

			//Since the first transaction should be finished,
			//the other element should also have been added.
			assert.True(t, bool(m.Has(ctx2, INT_1)))
			assert.True(t, bool(m.Contains(ctx2, STRING_A)))
			entryValue, ok := m.Get(ctx2, INT_1)
			if assert.True(t, bool(ok)) {
				assert.Equal(t, STRING_A, entryValue)
			}

			assert.True(t, bool(m.Has(ctx2, INT_2)))
			assert.True(t, bool(m.Contains(ctx2, STRING_B)))
			entryValue, ok = m.Get(ctx2, INT_2)
			if assert.True(t, bool(ok)) {
				assert.Equal(t, STRING_B, entryValue)
			}

			values := core.IterateAllValuesOnly(ctx1, m.Iterator(ctx1, core.IteratorConfiguration{}))
			assert.ElementsMatch(t, []any{STRING_A, STRING_B}, values)

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

		pattern := NewMapPattern(MapConfig{})

		storage.SetSerialized(ctx1, "/map", `[]`)
		val, err := loadMap(ctx1, core.FreeEntityLoadingParams{
			Key: "/map", Storage: storage, Pattern: pattern,
		})

		m := val.(*Map)

		//First transaction.

		m.Share(ctx1.GetClosestState())

		if !assert.NoError(t, err) {
			return
		}

		m.Set(ctx1, INT_1, STRING_A)
		if !assert.NoError(t, tx1.Commit(ctx1)) {
			return
		}

		//Second transaction.

		assert.True(t, bool(m.Has(ctx2, INT_1)))

		m.Set(ctx2, INT_2, STRING_B)
		assert.NoError(t, tx2.Commit(ctx2))

		//Check that the Map is persised

		persisted, err := loadMap(ctx2, core.FreeEntityLoadingParams{
			Key: "/map", Storage: storage, Pattern: pattern,
		})

		if !assert.NoError(t, err) {
			return
		}

		assert.NotSame(t, persisted, m) //future-proofing the test
		assert.True(t, bool(persisted.(*Map).Has(ctx2, INT_1)))
		assert.True(t, bool(persisted.(*Map).Contains(ctx2, STRING_A)))
		assert.True(t, bool(persisted.(*Map).Has(ctx2, INT_2)))
		assert.True(t, bool(persisted.(*Map).Contains(ctx2, STRING_B)))
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

	// 	storage.SetSerialized(ctx1, "/map", `[]`)
	// 	val, err := loadMap(ctx1, core.FreeEntityLoadingParams{
	// 		Key: "/map", Storage: storage, Pattern: pattern,
	// 	})

	// 	if !assert.NoError(t, err) {
	// 		return
	// 	}

	// 	m := val.(*Map)
	// 	m.Share(ctx1.GetClosestState())

	// 	var (
	// 		OBJ_1 = core.NewObjectFromMapNoInit(core.ValMap{})
	// 		OBJ_2 = core.NewObjectFromMapNoInit(core.ValMap{})
	// 	)

	// 	m.Add(ctx1, OBJ_1)
	// 	//Check that 1 is added from all POVs (it's okay).
	// 	assert.True(t, bool(m.Has(ctx1, OBJ_1)))
	// 	assert.True(t, bool(m.Has(ctx2, OBJ_1)))

	// 	m.Add(ctx2, OBJ_2)
	// 	//Check that 2 is added from all POVs (it's okay).
	// 	assert.True(t, bool(m.Has(ctx2, OBJ_2)))
	// 	assert.True(t, bool(m.Has(ctx1, OBJ_2)))

	// 	assert.NoError(t, tx1.Commit(ctx1))

	// 	//Check that 1 is added from ctx2's POV.
	// 	assert.True(t, bool(m.Has(ctx2, OBJ_1)))

	// 	//Check that 2 is still added from ctx2's POV.
	// 	assert.True(t, bool(m.Has(ctx2, OBJ_2)))
	// })

}

func TestSharedPersistedMapRemove(t *testing.T) {

	t.Run("Set should be persisted during call to .Remove", func(t *testing.T) {
		ctx, storage := sharedSetTestSetup(t)
		defer ctx.CancelGracefully()

		pattern := NewMapPattern(MapConfig{})

		storage.SetSerialized(ctx, "/map", `[{"int__value":"1"},"a"]`)
		val, err := loadMap(ctx, core.FreeEntityLoadingParams{
			Key: "/map", Storage: storage, Pattern: pattern,
		})

		if !assert.NoError(t, err) {
			return
		}

		m := val.(*Map)
		m.Share(ctx.GetClosestState())
		assert.True(t, bool(m.Has(ctx, INT_1)))

		m.Remove(ctx, INT_1)

		//Check that entry is removed.

		assert.False(t, bool(m.Has(ctx, INT_1)))
		_, ok := m.Get(ctx, INT_1)
		assert.False(t, bool(ok))

		values := core.IterateAllValuesOnly(ctx, m.Iterator(ctx, core.IteratorConfiguration{}))
		assert.Empty(t, values)

		//Check that the Map is persised

		persisted, err := loadMap(ctx, core.FreeEntityLoadingParams{
			Key: "/map", Storage: storage, Pattern: pattern,
		})

		if !assert.NoError(t, err) {
			return
		}

		assert.NotSame(t, persisted, m) //future-proofing the test

		vals := core.IterateAllValuesOnly(ctx, m.Iterator(ctx, core.IteratorConfiguration{}))
		assert.Len(t, vals, 0)
	})

	t.Run("Set should be persisted at end of successful transaction if .Remove was called transactionnaly", func(t *testing.T) {

		ctx, storage := sharedSetTestSetup(t)
		defer ctx.CancelGracefully()

		tx := core.StartNewTransaction(ctx)

		pattern := NewMapPattern(MapConfig{})

		storage.SetSerialized(ctx, "/map", `[{"int__value":"1"},"a"]`)
		val, err := loadMap(ctx, core.FreeEntityLoadingParams{
			Key: "/map", Storage: storage, Pattern: pattern,
		})

		if !assert.NoError(t, err) {
			return
		}

		m := val.(*Map)
		m.Share(ctx.GetClosestState())

		m.Remove(ctx, INT_1)

		//Check that entry is removed from the tx's POV.

		assert.False(t, bool(m.Has(ctx, INT_1)))
		_, ok := m.Get(ctx, INT_1)
		assert.False(t, bool(ok))

		values := core.IterateAllValuesOnly(ctx, m.Iterator(ctx, core.IteratorConfiguration{}))
		assert.Empty(t, values)

		//Check that the Map is not persised

		persisted, err := loadMap(ctx, core.FreeEntityLoadingParams{
			Key: "/map", Storage: storage, Pattern: pattern,
		})

		if !assert.NoError(t, err) {
			return
		}

		persistedMap := persisted.(*Map)

		assert.NotSame(t, persistedMap, m) //future-proofing the test
		assert.True(t, bool(persistedMap.Has(ctx, INT_1)))
		assert.True(t, bool(persistedMap.Contains(ctx, STRING_A)))

		//Commit the transaction.

		assert.NoError(t, tx.Commit(ctx))

		//Check that the Map is persised.

		persisted, err = loadMap(ctx, core.FreeEntityLoadingParams{
			Key: "/map", Storage: storage, Pattern: pattern,
		})

		if !assert.NoError(t, err) {
			return
		}

		persistedMap = persisted.(*Map)

		assert.NotSame(t, persisted, m) //future-proofing the test
		assert.False(t, bool(persistedMap.Has(ctx, INT_1)))
		assert.False(t, bool(persistedMap.Contains(ctx, STRING_A)))
	})

	t.Run("Set should not be persisted at end of failed transaction if .Remove was called transactionnaly", func(t *testing.T) {
		ctx1, ctx2, storage := sharedSetTestSetup2(t)
		defer ctx2.CancelGracefully()

		pattern := NewMapPattern(MapConfig{})

		storage.SetSerialized(ctx1, "/map", `[{"int__value":1},"a"]`)
		val, err := loadMap(ctx1, core.FreeEntityLoadingParams{
			Key: "/map", Storage: storage, Pattern: pattern,
		})

		m := val.(*Map)
		m.Share(ctx1.GetClosestState())

		//The tx is started after the KV write in order
		//for the write to be already commited.
		tx := core.StartNewTransaction(ctx1)

		if !assert.NoError(t, err) {
			return
		}

		if !assert.True(t, bool(m.Has(ctx1, INT_1))) {
			return
		}
		assert.True(t, bool(m.Contains(ctx1, STRING_A)))

		m.Remove(ctx1, INT_1)

		//Check that entry is removed from the tx's POV.

		assert.False(t, bool(m.Has(ctx1, INT_1)))
		_, ok := m.Get(ctx1, INT_1)
		assert.False(t, bool(ok))

		values := core.IterateAllValuesOnly(ctx1, m.Iterator(ctx1, core.IteratorConfiguration{}))
		assert.Empty(t, values)

		//Check that the Map is not persised

		persisted, err := loadMap(ctx1, core.FreeEntityLoadingParams{
			Key: "/map", Storage: storage, Pattern: pattern,
		})

		if !assert.NoError(t, err) {
			return
		}

		assert.NotSame(t, persisted, m) //future-proofing the test
		assert.True(t, bool(persisted.(*Map).Has(ctx1, INT_1)))
		assert.True(t, bool(persisted.(*Map).Contains(ctx1, STRING_A)))

		//roll back

		assert.NoError(t, tx.Rollback(ctx1))

		//Check that the Map has not been updated from another transaction's POV.

		core.StartNewTransaction(ctx2)
		assert.True(t, bool(m.Has(ctx2, INT_1)))
		assert.True(t, bool(m.Contains(ctx2, STRING_A)))

		//Check that the Map is not persised

		persisted, err = loadMap(ctx1, core.FreeEntityLoadingParams{
			Key: "/map", Storage: storage, Pattern: pattern,
		})

		if !assert.NoError(t, err) {
			return
		}

		assert.NotSame(t, persisted, m) //future-proofing the test
		assert.True(t, bool(persisted.(*Map).Has(ctx1, INT_1)))
		assert.True(t, bool(persisted.(*Map).Contains(ctx1, STRING_A)))
	})

	//Tests with several transactions.

	t.Run("write transactions should wait for the previous transaction to finish", func(t *testing.T) {
		ctx1, ctx2, storage := sharedSetTestSetup2(t)
		defer ctx1.CancelGracefully()
		defer ctx2.CancelGracefully()

		tx1 := core.StartNewTransaction(ctx1)
		core.StartNewTransaction(ctx2)

		pattern := NewMapPattern(MapConfig{})

		storage.SetSerialized(ctx1, "/map", `[{"int__value":1}, "a", {"int__value":2}, "b"]`)
		val, err := loadMap(ctx1, core.FreeEntityLoadingParams{
			Key: "/map", Storage: storage, Pattern: pattern,
		})

		if !assert.NoError(t, err) {
			return
		}

		m := val.(*Map)
		m.Share(ctx1.GetClosestState())

		m.Remove(ctx1, INT_1)

		//Check that entry is removed from tx1's POV.

		assert.False(t, bool(m.Has(ctx1, INT_1)))
		_, ok := m.Get(ctx1, INT_1)
		assert.False(t, bool(ok))

		values := core.IterateAllValuesOnly(ctx1, m.Iterator(ctx1, core.IteratorConfiguration{}))
		assert.ElementsMatch(t, []any{STRING_B}, values)

		tx2Done := make(chan struct{})
		go func() { //second transaction
			m.Remove(ctx2, INT_2)

			//Since the first transaction should be finished,
			//the other element should also have been removed.
			assert.False(t, bool(m.Has(ctx2, INT_1)))
			assert.False(t, bool(m.Contains(ctx2, STRING_A)))
			_, ok := m.Get(ctx2, INT_1)
			assert.False(t, bool(ok))

			assert.False(t, bool(m.Has(ctx2, INT_2)))
			assert.False(t, bool(m.Contains(ctx2, STRING_B)))
			_, ok = m.Get(ctx2, INT_2)
			assert.False(t, bool(ok))

			values := core.IterateAllValuesOnly(ctx1, m.Iterator(ctx1, core.IteratorConfiguration{}))
			assert.Empty(t, values)

			tx2Done <- struct{}{}
		}()

		assert.NoError(t, tx1.Commit(ctx1))
		<-tx2Done
	})

}

func TestSharedPersistedSetHas(t *testing.T) {

	//Tests with several transactions.

	t.Run("readonly transactions can read the Map in parallel", func(t *testing.T) {
		ctx1, ctx2, storage := sharedSetTestSetup2(t)
		defer ctx1.CancelGracefully()
		defer ctx2.CancelGracefully()

		readTx1 := core.StartNewReadonlyTransaction(ctx1)
		core.StartNewReadonlyTransaction(ctx2)

		pattern := NewMapPattern(MapConfig{})

		storage.SetSerialized(ctx1, "/map", `[{"int__value":1},"a"]`)
		val, err := loadMap(ctx1, core.FreeEntityLoadingParams{
			Key: "/map", Storage: storage, Pattern: pattern,
		})

		if !assert.NoError(t, err) {
			return
		}

		m := val.(*Map)
		m.Share(ctx1.GetClosestState())

		assert.True(t, bool(m.Has(ctx1, INT_1)))
		assert.True(t, bool(m.Has(ctx2, INT_1)))

		assert.True(t, bool(m.Has(ctx1, INT_1)))
		assert.True(t, bool(m.Has(ctx2, INT_1)))

		assert.NoError(t, readTx1.Commit(ctx1))
		assert.True(t, bool(m.Has(ctx2, INT_1)))
	})
}

func TestInteractWithElementsOfLoadedSet(t *testing.T) {

}
