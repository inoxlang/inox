package setcoll

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/inoxlang/inox/internal/commonfmt"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/filekv"
	"github.com/inoxlang/inox/internal/globals/containers/common"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/stretchr/testify/assert"
)

func TestPersistLoadSet(t *testing.T) {
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

		pattern := NewSetPattern(SetConfig{
			Uniqueness: common.UniquenessConstraint{
				Type: common.UniqueRepr,
			},
		})
		set := NewSetWithConfig(ctx, nil, pattern.config)

		//persist
		{
			persistSet(ctx, set, "/set", storage)

			serialized, ok := storage.GetSerialized(ctx, "/set")
			if !assert.True(t, ok) {
				return
			}
			assert.Equal(t, "[]", serialized)
		}

		loadedSet, err := loadSet(ctx, core.FreeEntityLoadingParams{
			Key: "/set", Storage: storage, Pattern: pattern,
		})
		if !assert.NoError(t, err) {
			return
		}

		if !assert.NotNil(t, loadedSet) {
			return
		}

		//set should be shared
		assert.True(t, loadedSet.(*Set).IsShared())
	})

	t.Run("unique repr: single element", func(t *testing.T) {
		ctx, storage := setup()
		defer ctx.CancelGracefully()

		pattern := NewSetPattern(SetConfig{
			Uniqueness: common.UniquenessConstraint{
				Type: common.UniqueRepr,
			},
		})
		set := NewSetWithConfig(ctx, nil, pattern.config)

		int1 := core.Int(1)
		set.Add(ctx, int1)

		//persist
		{
			persistSet(ctx, set, "/set", storage)

			serialized, ok := storage.GetSerialized(ctx, "/set")
			if !assert.True(t, ok) {
				return
			}
			assert.Equal(t, `[{"int__value":1}]`, serialized)
		}

		loaded, err := loadSet(ctx, core.FreeEntityLoadingParams{
			Key: "/set", Storage: storage, Pattern: pattern,
		})
		if !assert.NoError(t, err) {
			return
		}

		loadedSet := loaded.(*Set)

		//set should be shared
		if !assert.True(t, loadedSet.IsShared()) {
			return
		}

		//check elements
		assert.True(t, bool(loadedSet.Has(ctx, int1)))
		assert.True(t, bool(loadedSet.Contains(ctx, int1)))

		elemKey := loadedSet.getElementPathKeyFromKey(`{"int__value":1}`)
		elem, err := loadedSet.GetElementByKey(ctx, elemKey)
		if !assert.NoError(t, err, core.ErrCollectionElemNotFound) {
			return
		}
		assert.Equal(t, int1, elem)
	})

	t.Run("unique repr: two elements", func(t *testing.T) {
		ctx, storage := setup()
		defer ctx.CancelGracefully()

		pattern := NewSetPattern(SetConfig{
			Uniqueness: common.UniquenessConstraint{
				Type: common.UniqueRepr,
			},
			Element: core.SERIALIZABLE_PATTERN,
		})
		set := NewSetWithConfig(ctx, nil, pattern.config)

		int1 := core.Int(1)
		int2 := core.Int(2)

		set.Add(ctx, int1)
		set.Add(ctx, int2)

		//persist
		{
			persistSet(ctx, set, "/set", storage)

			serialized, ok := storage.GetSerialized(ctx, "/set")
			if !assert.True(t, ok) {
				return
			}
			assert.Regexp(t, `(\[{"int__value":1},{"int__value":2}]|\[{"int__value":2},{"int__value":1}])`, serialized)
		}

		loaded, err := loadSet(ctx, core.FreeEntityLoadingParams{
			Key: "/set", Storage: storage, Pattern: pattern,
		})
		if !assert.NoError(t, err) {
			return
		}

		loadedSet := loaded.(*Set)

		//set should be shared
		if !assert.True(t, loadedSet.IsShared()) {
			return
		}

		//check elements
		assert.True(t, bool(loadedSet.Has(ctx, int1)))
		assert.True(t, bool(loadedSet.Contains(ctx, int1)))

		assert.True(t, bool(loadedSet.Has(ctx, int2)))
		assert.True(t, bool(loadedSet.Contains(ctx, int2)))

		elem1Key := loadedSet.getElementPathKeyFromKey(`{"int__value":1}`)
		elem1, err := loadedSet.GetElementByKey(ctx, elem1Key)
		if !assert.NoError(t, err) {
			return
		}
		assert.Equal(t, int1, elem1)

		elem2Key := loadedSet.getElementPathKeyFromKey(`{"int__value":2}`)
		elem2, err := loadedSet.GetElementByKey(ctx, elem2Key)
		if !assert.NoError(t, err) {
			return
		}
		assert.Equal(t, int2, elem2)
	})

	t.Run("unique repr: element with non-unique repr", func(t *testing.T) {
		ctx, storage := setup()
		defer ctx.CancelGracefully()

		pattern := NewSetPattern(SetConfig{
			Uniqueness: common.UniquenessConstraint{
				Type: common.UniqueRepr,
			},
		})

		//a mutable object is not considered to have a unique representation.

		storage.SetSerialized(ctx, "/set", `[{"object__value":{}}]`)
		set, err := loadSet(ctx, core.FreeEntityLoadingParams{
			Key: "/set", Storage: storage, Pattern: pattern,
		})
		if !assert.ErrorIs(t, err, core.ErrReprOfMutableValueCanChange) {
			return
		}

		assert.Nil(t, set)
	})

	t.Run("unique property value: element with missing property", func(t *testing.T) {
		ctx, storage := setup()
		defer ctx.CancelGracefully()

		pattern := NewSetPattern(SetConfig{
			Element: core.NewInexactObjectPattern([]core.ObjectPatternEntry{{Name: "id", Pattern: core.STRING_PATTERN}}),
			Uniqueness: common.UniquenessConstraint{
				Type:         common.UniquePropertyValue,
				PropertyName: "id",
			},
		})

		storage.SetSerialized(ctx, "/set", `[{}]`)
		set, err := loadSet(ctx, core.FreeEntityLoadingParams{
			Key: "/set", Storage: storage, Pattern: pattern,
		})
		if !assert.ErrorContains(t, err, "properties are missing") {
			return
		}

		assert.Nil(t, set)
	})

	t.Run("unique property value: one element", func(t *testing.T) {
		ctx, storage := setup()
		defer ctx.CancelGracefully()

		pattern := NewSetPattern(SetConfig{
			Element: core.NewInexactObjectPattern([]core.ObjectPatternEntry{{Name: "id", Pattern: core.STR_PATTERN}}),
			Uniqueness: common.UniquenessConstraint{
				Type:         common.UniquePropertyValue,
				PropertyName: "id",
			},
		})
		set := NewSetWithConfig(ctx, nil, pattern.config)

		set.Add(ctx, core.NewObjectFromMap(core.ValMap{"id": core.String("a")}, ctx))

		//persist
		{
			persistSet(ctx, set, "/set", storage)

			serialized, ok := storage.GetSerialized(ctx, "/set")
			if !assert.True(t, ok) {
				return
			}
			assert.Equal(t, `[{"id":"a"}]`, serialized)
		}

		loadedSet, err := loadSet(ctx, core.FreeEntityLoadingParams{
			Key: "/set", Storage: storage, Pattern: pattern,
		})
		if !assert.NoError(t, err) {
			return
		}

		if !assert.NotNil(t, loadedSet) {
			return
		}

		//set should be shared
		assert.True(t, loadedSet.(*Set).IsShared())
	})

	t.Run("unique property value: two elements", func(t *testing.T) {
		ctx, storage := setup()
		defer ctx.CancelGracefully()

		pattern := NewSetPattern(SetConfig{
			Element: core.NewInexactObjectPattern([]core.ObjectPatternEntry{{Name: "id", Pattern: core.STR_PATTERN}}),
			Uniqueness: common.UniquenessConstraint{
				Type:         common.UniquePropertyValue,
				PropertyName: "id",
			},
		})
		set := NewSetWithConfig(ctx, nil, pattern.config)

		set.Add(ctx, core.NewObjectFromMap(core.ValMap{"id": core.String("a")}, ctx))
		set.Add(ctx, core.NewObjectFromMap(core.ValMap{"id": core.String("b")}, ctx))

		//persist
		{
			persistSet(ctx, set, "/set", storage)

			serialized, ok := storage.GetSerialized(ctx, "/set")
			if !assert.True(t, ok) {
				return
			}
			if strings.Index(serialized, `"a"`) < strings.Index(serialized, `"b"`) {
				assert.Equal(t, `[{"id":"a"},{"id":"b"}]`, serialized)
			} else {
				assert.Equal(t, `[{"id":"b"},{"id":"a"}]`, serialized)
			}
		}

		loadedSet, err := loadSet(ctx, core.FreeEntityLoadingParams{
			Key: "/set", Storage: storage, Pattern: pattern,
		})
		if !assert.NoError(t, err) {
			return
		}

		if !assert.NotNil(t, loadedSet) {
			return
		}

		//set should be shared
		assert.True(t, loadedSet.(*Set).IsShared())
	})

	t.Run("unique property value: two elements with same unique prop", func(t *testing.T) {
		ctx, storage := setup()
		defer ctx.CancelGracefully()

		pattern := NewSetPattern(SetConfig{
			Element: core.NewInexactObjectPattern([]core.ObjectPatternEntry{{Name: "id", Pattern: core.STR_PATTERN}}),
			Uniqueness: common.UniquenessConstraint{
				Type:         common.UniquePropertyValue,
				PropertyName: "id",
			},
		})

		storage.SetSerialized(ctx, "/set", `[{"id": "a"}, {"id": "a"}]`)
		set, err := loadSet(ctx, core.FreeEntityLoadingParams{
			Key: "/set", Storage: storage, Pattern: pattern,
		})
		if !assert.ErrorIs(t, err, ErrCannotAddDifferentElemWithSamePropertyValue) {
			return
		}

		assert.Nil(t, set)
	})

	t.Run("migration: deletion", func(t *testing.T) {
		ctx := core.NewContextWithEmptyState(core.ContextConfig{}, nil)
		storage := &core.TestValueStorage{
			BaseURL_: "ldb://main/",
			Data:     map[core.Path]string{"/x": `[]`},
		}
		pattern := NewSetPattern(SetConfig{
			Uniqueness: common.UniquenessConstraint{
				Type: common.UniqueRepr,
			},
		})

		val, err := loadSet(ctx, core.FreeEntityLoadingParams{
			Key:          "/x",
			Storage:      storage,
			Pattern:      pattern,
			AllowMissing: false,
			Migration: &core.FreeEntityMigrationArgs{
				MigrationHandlers: core.MigrationOpHandlers{
					Deletions: map[core.PathPattern]*core.MigrationOpHandler{
						"/x": nil,
					},
				},
			},
		})

		if !assert.NoError(t, err) {
			return
		}

		assert.Nil(t, val)
	})

	t.Run("migration: replacement", func(t *testing.T) {
		ctx := core.NewContextWithEmptyState(core.ContextConfig{}, nil)
		storage := &core.TestValueStorage{
			BaseURL_: "ldb://main/",
			Data:     map[core.Path]string{"/x": `[]`},
		}
		pattern := NewSetPattern(SetConfig{
			Uniqueness: common.UniquenessConstraint{
				Type: common.UniqueRepr,
			},
		})
		nextPattern := core.NewInexactObjectPattern([]core.ObjectPatternEntry{
			{Name: "a", Pattern: core.INT_PATTERN},
		})

		val, err := loadSet(ctx, core.FreeEntityLoadingParams{
			Key:          "/x",
			Storage:      storage,
			Pattern:      pattern,
			AllowMissing: false,
			Migration: &core.FreeEntityMigrationArgs{
				NextPattern: nextPattern,
				MigrationHandlers: core.MigrationOpHandlers{
					Replacements: map[core.PathPattern]*core.MigrationOpHandler{
						"/x": {
							InitialValue: core.NewObjectFromMap(core.ValMap{"a": core.Int(1)}, ctx),
						},
					},
				},
			},
		})

		if !assert.NoError(t, err) {
			return
		}

		if !assert.NotNil(t, val) {
			return
		}
		object := val.(*core.Object)

		url, _ := object.URL()

		if !assert.Equal(t, core.URL("ldb://main/x"), url) {
			return
		}

		assert.True(t, object.IsShared())

		//make sure the post-migration value is saved
		assert.Equal(t, `{"_url_":"ldb://main/x","a":1}`, storage.Data["/x"])
	})
}

func TestSetMigrate(t *testing.T) {

	t.Run("delete Set: / key", func(t *testing.T) {
		config := SetConfig{
			Element:    core.SERIALIZABLE_PATTERN,
			Uniqueness: common.UniquenessConstraint{Type: common.UniqueRepr},
		}

		ctx := core.NewContextWithEmptyState(core.ContextConfig{}, nil)
		set := NewSetWithConfig(ctx, nil, config)
		val, err := set.Migrate(ctx, "/", &core.FreeEntityMigrationArgs{
			NextPattern: nil,
			MigrationHandlers: core.MigrationOpHandlers{
				Deletions: map[core.PathPattern]*core.MigrationOpHandler{
					"/": nil,
				},
			},
		})

		if !assert.NoError(t, err) {
			return
		}
		assert.Nil(t, val)
	})

	t.Run("delete Set: /x key", func(t *testing.T) {
		config := SetConfig{
			Element:    core.SERIALIZABLE_PATTERN,
			Uniqueness: common.UniquenessConstraint{Type: common.UniqueRepr},
		}

		ctx := core.NewContextWithEmptyState(core.ContextConfig{}, nil)
		set := NewSetWithConfig(ctx, nil, config)
		val, err := set.Migrate(ctx, "/x", &core.FreeEntityMigrationArgs{
			NextPattern: nil,
			MigrationHandlers: core.MigrationOpHandlers{
				Deletions: map[core.PathPattern]*core.MigrationOpHandler{
					"/x": nil,
				},
			},
		})

		if !assert.NoError(t, err) {
			return
		}
		assert.Nil(t, val)
	})

	t.Run("delete element", func(t *testing.T) {
		config := SetConfig{
			Element:    core.INT_PATTERN,
			Uniqueness: common.UniquenessConstraint{Type: common.UniqueRepr},
		}

		ctx := core.NewContextWithEmptyState(core.ContextConfig{}, nil)
		element := core.Int(0)
		set := NewSetWithConfig(ctx, core.NewWrappedValueList(element), config)

		val, err := set.Migrate(ctx, "/", &core.FreeEntityMigrationArgs{
			NextPattern: nil,
			MigrationHandlers: core.MigrationOpHandlers{
				Deletions: map[core.PathPattern]*core.MigrationOpHandler{
					"/" + core.PathPattern(common.GetElementPathKeyFromKey("0", common.UniqueRepr)): nil,
				},
			},
		})

		if !assert.NoError(t, err) {
			return
		}
		assert.Same(t, set, val)
		assert.Equal(t, map[string]core.Serializable{}, set.elementByKey)
	})

	t.Run("delete all elements", func(t *testing.T) {
		config := SetConfig{
			Element:    core.INT_PATTERN,
			Uniqueness: common.UniquenessConstraint{Type: common.UniqueRepr},
		}

		ctx := core.NewContextWithEmptyState(core.ContextConfig{}, nil)
		set := NewSetWithConfig(ctx, core.NewWrappedValueList(core.Int(0), core.Int(1)), config)

		val, err := set.Migrate(ctx, "/", &core.FreeEntityMigrationArgs{
			NextPattern: nil,
			MigrationHandlers: core.MigrationOpHandlers{
				Deletions: map[core.PathPattern]*core.MigrationOpHandler{
					"/*": nil,
				},
			},
		})

		if !assert.NoError(t, err) {
			return
		}
		assert.Same(t, set, val)
		assert.Equal(t, map[string]core.Serializable{}, set.elementByKey)
	})

	t.Run("delete inexisting element", func(t *testing.T) {
		config := SetConfig{
			Element:    core.INT_PATTERN,
			Uniqueness: common.UniquenessConstraint{Type: common.UniqueRepr},
		}

		ctx := core.NewContextWithEmptyState(core.ContextConfig{}, nil)
		set := NewSetWithConfig(ctx, core.NewWrappedValueList(core.Int(0)), config)

		pathPattern := "/" + core.PathPattern(common.GetElementPathKeyFromKey("1", common.UniqueRepr))

		val, err := set.Migrate(ctx, "/", &core.FreeEntityMigrationArgs{
			NextPattern: nil,
			MigrationHandlers: core.MigrationOpHandlers{
				Deletions: map[core.PathPattern]*core.MigrationOpHandler{
					pathPattern: nil,
				},
			},
		})

		if !assert.Equal(t, err, commonfmt.FmtValueAtPathDoesNotExist(string(pathPattern))) {
			return
		}
		assert.Nil(t, val)
	})

	t.Run("delete property of element", func(t *testing.T) {
		config := SetConfig{
			Element:    core.RECORD_PATTERN,
			Uniqueness: common.UniquenessConstraint{Type: common.UniqueRepr},
		}

		ctx := core.NewContextWithEmptyState(core.ContextConfig{}, nil)
		elements := core.NewWrappedValueList(core.NewRecordFromMap(core.ValMap{"b": core.Int(0)}))
		set := NewSetWithConfig(ctx, elements, config)

		pathPattern := "/" + core.PathPattern(common.GetElementPathKeyFromKey(`{"b":{"int__value":0}}`, common.UniqueRepr)) + "/b"

		val, err := set.Migrate(ctx, "/", &core.FreeEntityMigrationArgs{
			NextPattern: nil,
			MigrationHandlers: core.MigrationOpHandlers{
				Deletions: map[core.PathPattern]*core.MigrationOpHandler{
					pathPattern: nil,
				},
			},
		})

		if !assert.NoError(t, err) {
			return
		}
		assert.Same(t, set, val)
		expectedElement := core.NewRecordFromMap(core.ValMap{})
		assert.Equal(t, map[string]core.Serializable{"{}": expectedElement}, set.elementByKey)
	})

	t.Run("replace Set: / key", func(t *testing.T) {
		config := SetConfig{
			Element:    core.SERIALIZABLE_PATTERN,
			Uniqueness: common.UniquenessConstraint{Type: common.UniqueRepr},
		}

		ctx := core.NewContextWithEmptyState(core.ContextConfig{}, nil)
		set := NewSetWithConfig(ctx, core.NewWrappedValueList(), config)
		replacement := core.NewWrappedValueList()

		val, err := set.Migrate(ctx, "/", &core.FreeEntityMigrationArgs{
			NextPattern: NewSetPattern(config),
			MigrationHandlers: core.MigrationOpHandlers{
				Replacements: map[core.PathPattern]*core.MigrationOpHandler{
					"/": {InitialValue: replacement},
				},
			},
		})

		if !assert.NoError(t, err) {
			return
		}
		if !assert.Equal(t, replacement, val) {
			return
		}
		assert.NotSame(t, replacement, val)
	})

	t.Run("replace Set: /x key", func(t *testing.T) {
		config := SetConfig{
			Element:    core.SERIALIZABLE_PATTERN,
			Uniqueness: common.UniquenessConstraint{Type: common.UniqueRepr},
		}

		ctx := core.NewContextWithEmptyState(core.ContextConfig{}, nil)
		set := NewSetWithConfig(ctx, core.NewWrappedValueList(), config)
		replacement := core.NewWrappedValueList()

		val, err := set.Migrate(ctx, "/x", &core.FreeEntityMigrationArgs{
			NextPattern: core.NewListPatternOf(core.ANYVAL_PATTERN),
			MigrationHandlers: core.MigrationOpHandlers{
				Replacements: map[core.PathPattern]*core.MigrationOpHandler{
					"/x": {InitialValue: replacement},
				},
			},
		})

		if !assert.NoError(t, err) {
			return
		}
		if !assert.Equal(t, replacement, val) {
			return
		}
		assert.NotSame(t, replacement, val)
	})

	t.Run("replace property of immutable elements", func(t *testing.T) {
		config := SetConfig{
			Element:    core.SERIALIZABLE_PATTERN,
			Uniqueness: common.UniquenessConstraint{Type: common.UniqueRepr},
		}

		ctx := core.NewContextWithEmptyState(core.ContextConfig{}, nil)
		elements := core.NewWrappedValueList(
			core.NewRecordFromMap(core.ValMap{"a": core.Int(1), "b": core.Int(1)}),
			core.NewRecordFromMap(core.ValMap{"a": core.Int(2), "b": core.Int(2)}),
		)

		set := NewSetWithConfig(ctx, elements, config)
		val, err := set.Migrate(ctx, "/", &core.FreeEntityMigrationArgs{
			NextPattern: nil,
			MigrationHandlers: core.MigrationOpHandlers{
				Replacements: map[core.PathPattern]*core.MigrationOpHandler{
					`/*/b`: {InitialValue: core.Int(3)},
				},
			},
		})

		if !assert.NoError(t, err) {
			return
		}
		if !assert.Same(t, set, val) {
			return
		}

		expectedRecord1 := core.NewRecordFromMap(core.ValMap{"a": core.Int(1), "b": core.Int(3)})
		expectedRecord2 := core.NewRecordFromMap(core.ValMap{"a": core.Int(2), "b": core.Int(3)})

		assert.Equal(t, map[string]core.Serializable{
			string(core.ToJSON(ctx, expectedRecord1, nil)): expectedRecord1,
			string(core.ToJSON(ctx, expectedRecord2, nil)): expectedRecord2,
		}, set.elementByKey)
	})

	t.Run("element inclusion should panic", func(t *testing.T) {
		config := SetConfig{
			Element:    core.SERIALIZABLE_PATTERN,
			Uniqueness: common.UniquenessConstraint{Type: common.UniqueRepr},
		}

		ctx := core.NewContextWithEmptyState(core.ContextConfig{}, nil)

		set := NewSetWithConfig(ctx, nil, config)

		assert.PanicsWithError(t, core.ErrUnreachable.Error(), func() {
			set.Migrate(ctx, "/", &core.FreeEntityMigrationArgs{
				NextPattern: nil,
				MigrationHandlers: core.MigrationOpHandlers{
					Inclusions: map[core.PathPattern]*core.MigrationOpHandler{
						"/0": {InitialValue: core.Int(1)},
					},
				},
			})
		})
	})
}
