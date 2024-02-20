package mapcoll

import (
	"path/filepath"
	"testing"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/filekv"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/stretchr/testify/assert"
)

func TestPersistLoadMap(t *testing.T) {
	setup := func() (*core.Context, core.DataStore) {
		ctx := core.NewContextWithEmptyState(core.ContextConfig{}, nil)
		kv := utils.Must(filekv.OpenSingleFileKV(filekv.KvStoreConfig{
			Path: core.PathFrom(filepath.Join(t.TempDir(), "data.kv")),
		}))
		storage := filekv.NewSerializedValueStorage(kv, "ldb://main/")
		return ctx, storage
	}

	t.Run("empty", func(t *testing.T) {
		ctx, storage := setup()
		defer ctx.CancelGracefully()

		pattern := NewMapPattern(MapConfig{})
		m := NewMapWithConfig(ctx, nil, pattern.config)

		//persist
		{
			persistMap(ctx, m, "/map", storage)

			serialized, ok := storage.GetSerialized(ctx, "/map")
			if !assert.True(t, ok) {
				return
			}
			assert.Equal(t, "[]", serialized)
		}

		loadedMap, err := loadMap(ctx, core.FreeEntityLoadingParams{
			Key: "/map", Storage: storage, Pattern: pattern,
		})
		if !assert.NoError(t, err) {
			return
		}

		if !assert.NotNil(t, loadedMap) {
			return
		}

		//map should be shared
		assert.True(t, loadedMap.(*Map).IsShared())
	})

	t.Run("single entry", func(t *testing.T) {
		ctx, storage := setup()
		defer ctx.CancelGracefully()

		pattern := NewMapPattern(MapConfig{})
		m := NewMapWithConfig(ctx, nil, pattern.config)

		INT_1 := core.Int(1)
		m.Insert(ctx, INT_1, STRING_A)

		//persist
		{
			persistMap(ctx, m, "/map", storage)

			serialized, ok := storage.GetSerialized(ctx, "/map")
			if !assert.True(t, ok) {
				return
			}
			assert.Equal(t, `[{"int__value":1},"a"]`, serialized)
		}

		loaded, err := loadMap(ctx, core.FreeEntityLoadingParams{
			Key: "/map", Storage: storage, Pattern: pattern,
		})
		if !assert.NoError(t, err) {
			return
		}

		loadedMap := loaded.(*Map)

		//map should be shared
		if !assert.True(t, loadedMap.IsShared()) {
			return
		}

		//check elements
		assert.True(t, bool(loadedMap.Has(ctx, INT_1)))
		assert.True(t, bool(loadedMap.Contains(ctx, STRING_A)))
		assert.False(t, bool(loadedMap.Contains(ctx, INT_1)))

		//TODO
		// elemKey := loadedMap.getElementPathKeyFromKey(`{"int__value":1}`)
		// elem, err := loadedMap.GetElementByKey(ctx, elemKey)
		// if !assert.NoError(t, err, core.ErrCollectionElemNotFound) {
		// 	return
		// }
		//assert.Equal(t, INT_1, elem)
	})

	t.Run("two entries", func(t *testing.T) {
		ctx, storage := setup()
		defer ctx.CancelGracefully()

		pattern := NewMapPattern(MapConfig{})
		m := NewMapWithConfig(ctx, nil, pattern.config)

		m.Insert(ctx, INT_1, STRING_A)
		m.Insert(ctx, INT_2, STRING_B)
		//persist
		{
			persistMap(ctx, m, "/map", storage)

			serialized, ok := storage.GetSerialized(ctx, "/map")
			if !assert.True(t, ok) {
				return
			}
			assert.Regexp(t, `(\[{"int__value":1},"a",{"int__value":2},"b"]|\[{"int__value":2},"b",{"int__value":1},"a"])`, serialized)
		}

		loaded, err := loadMap(ctx, core.FreeEntityLoadingParams{
			Key: "/map", Storage: storage, Pattern: pattern,
		})
		if !assert.NoError(t, err) {
			return
		}

		loadedMap := loaded.(*Map)

		//map should be shared
		if !assert.True(t, loadedMap.IsShared()) {
			return
		}

		//check elements
		assert.True(t, bool(loadedMap.Has(ctx, INT_1)))
		assert.True(t, bool(loadedMap.Contains(ctx, STRING_A)))
		assert.False(t, bool(loadedMap.Contains(ctx, INT_1)))

		assert.True(t, bool(loadedMap.Has(ctx, INT_2)))
		assert.True(t, bool(loadedMap.Contains(ctx, STRING_B)))
		assert.False(t, bool(loadedMap.Contains(ctx, INT_2)))

		//TODO
		// elem1Key := loadedMap.getElementPathKeyFromKey(`{"int__value":1}`)
		// elem1, err := loadedMap.GetElementByKey(ctx, elem1Key)
		// if !assert.NoError(t, err) {
		// 	return
		// }
		// assert.Equal(t, INT_1, elem1)

		// elem2Key := loadedMap.getElementPathKeyFromKey(`{"int__value":2}`)
		// elem2, err := loadedMap.GetElementByKey(ctx, elem2Key)
		// if !assert.NoError(t, err) {
		// 	return
		// }
		// assert.Equal(t, INT_2, elem2)
	})

	t.Run("mutable key", func(t *testing.T) {
		ctx, storage := setup()
		defer ctx.CancelGracefully()

		pattern := NewMapPattern(MapConfig{})

		//a mutable object is not considered to have a unique representation.

		storage.SetSerialized(ctx, "/map", `[{"object__value":{}},"a"]`)
		m, err := loadMap(ctx, core.FreeEntityLoadingParams{
			Key: "/map", Storage: storage, Pattern: pattern,
		})
		if !assert.ErrorIs(t, err, ErrKeysShouldBeImmutable) {
			return
		}

		assert.Nil(t, m)
	})

	t.Run("migration: deletion", func(t *testing.T) {
		//TODO
	})

	t.Run("migration: replacement", func(t *testing.T) {
		//TODO
	})
}

func TestSetMigrate(t *testing.T) {

	//TODO
}
