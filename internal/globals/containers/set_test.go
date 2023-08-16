package containers

import (
	"io"
	"strings"
	"testing"

	"github.com/inoxlang/inox/internal/commonfmt"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/filekv"
	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/stretchr/testify/assert"

	containers_common "github.com/inoxlang/inox/internal/globals/containers/common"
	coll_symbolic "github.com/inoxlang/inox/internal/globals/containers/symbolic"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
)

const (
	MAX_MEM_FS_SIZE = 10_000
)

func TestNewSet(t *testing.T) {

	t.Run("no elements", func(t *testing.T) {
		ctx := core.NewContexWithEmptyState(core.ContextConfig{}, io.Discard)
		set := NewSet(ctx, core.NewWrappedValueList())

		assert.Equal(t, SetConfig{
			Uniqueness: containers_common.UniquenessConstraint{
				Type: containers_common.UniqueRepr,
			},
			Element: core.SERIALIZABLE_PATTERN,
		}, set.config)
	})

	t.Run("single element", func(t *testing.T) {
		ctx := core.NewContexWithEmptyState(core.ContextConfig{}, io.Discard)
		set := NewSet(ctx, core.NewWrappedValueList(core.Int(1)))

		assert.Equal(t, SetConfig{
			Uniqueness: containers_common.UniquenessConstraint{
				Type: containers_common.UniqueRepr,
			},
			Element: core.SERIALIZABLE_PATTERN,
		}, set.config)
	})

	t.Run("element with no representation", func(t *testing.T) {
		ctx := core.NewContexWithEmptyState(core.ContextConfig{}, io.Discard)

		node := core.AstNode{Node: parse.MustParseChunk("")}

		func() {
			defer func() {
				assert.ErrorContains(t, recover().(error), "failed to get representation")
			}()
			NewSet(ctx, core.NewWrappedValueList(node))
		}()
	})

	t.Run("element with representation should be immutable ", func(t *testing.T) {
		ctx := core.NewContexWithEmptyState(core.ContextConfig{}, io.Discard)

		obj := core.NewObjectFromMap(core.ValMap{}, ctx)

		func() {
			defer func() {
				assert.ErrorContains(t, recover().(error), core.ErrReprOfMutableValueCanChange.Error())
			}()
			NewSet(ctx, core.NewWrappedValueList(obj))
		}()
	})

	t.Run("url uniqueness", func(t *testing.T) {
		ctx := core.NewContexWithEmptyState(core.ContextConfig{}, io.Discard)

		config := core.NewObjectFromMap(core.ValMap{
			coll_symbolic.SET_CONFIG_UNIQUE_PROP_KEY: core.Identifier("url"),
		}, ctx)

		set := NewSet(ctx, core.NewWrappedValueList(), config)

		assert.Equal(t, SetConfig{
			Uniqueness: containers_common.UniquenessConstraint{
				Type: containers_common.UniqueURL,
			},
			Element: core.SERIALIZABLE_PATTERN,
		}, set.config)
	})

	t.Run("url uniqueness: element has no URL & Set has no URL", func(t *testing.T) {
		ctx := core.NewContexWithEmptyState(core.ContextConfig{}, io.Discard)

		config := core.NewObjectFromMap(core.ValMap{
			coll_symbolic.SET_CONFIG_UNIQUE_PROP_KEY: core.Identifier("url"),
		}, ctx)

		func() {
			defer func() {
				assert.ErrorContains(t, recover().(error), containers_common.ErrFailedGetUniqueKeyNoURL.Error())
			}()
			NewSet(ctx, core.NewWrappedValueList(core.NewObjectFromMap(nil, ctx)), config)
		}()

	})

	t.Run("uniqueness of property's value", func(t *testing.T) {
		ctx := core.NewContexWithEmptyState(core.ContextConfig{}, io.Discard)

		config := core.NewObjectFromMap(core.ValMap{
			coll_symbolic.SET_CONFIG_UNIQUE_PROP_KEY: core.PropertyName("id"),
		}, ctx)

		set := NewSet(ctx, core.NewWrappedValueList(), config)

		assert.Equal(t, SetConfig{
			Uniqueness: containers_common.UniquenessConstraint{
				Type:         containers_common.UniquePropertyValue,
				PropertyName: "id",
			},
			Element: core.SERIALIZABLE_PATTERN,
		}, set.config)
	})

	t.Run("uniqueness of property's value: element has no properties", func(t *testing.T) {
		ctx := core.NewContexWithEmptyState(core.ContextConfig{}, io.Discard)

		config := core.NewObjectFromMap(core.ValMap{
			coll_symbolic.SET_CONFIG_UNIQUE_PROP_KEY: core.PropertyName("id"),
		}, ctx)

		func() {
			defer func() {
				assert.ErrorContains(t, recover().(error), containers_common.ErrFailedGetUniqueKeyNoProps.Error())
			}()
			NewSet(ctx, core.NewWrappedValueList(core.Int(1)), config)
		}()
	})

	t.Run("element pattern", func(t *testing.T) {
		ctx := core.NewContexWithEmptyState(core.ContextConfig{}, io.Discard)

		elementPattern := core.NewInexactObjectPattern(map[string]core.Pattern{
			"a": core.INT_PATTERN,
		})

		config := core.NewObjectFromMap(core.ValMap{
			coll_symbolic.SET_CONFIG_ELEMENT_PATTERN_PROP_KEY: elementPattern,
		}, ctx)

		set := NewSet(ctx, core.NewWrappedValueList(), config)

		assert.Equal(t, SetConfig{
			Uniqueness: containers_common.UniquenessConstraint{
				Type: containers_common.UniqueRepr,
			},
			Element: elementPattern,
		}, set.config)
	})

	t.Run("element pattern: element does not match", func(t *testing.T) {
		ctx := core.NewContexWithEmptyState(core.ContextConfig{}, io.Discard)

		elementPattern := core.NewInexactObjectPattern(map[string]core.Pattern{
			"a": core.INT_PATTERN,
		})

		config := core.NewObjectFromMap(core.ValMap{
			coll_symbolic.SET_CONFIG_ELEMENT_PATTERN_PROP_KEY: elementPattern,
		}, ctx)

		func() {
			defer func() {
				assert.ErrorContains(t, recover().(error), ErrValueDoesMatchElementPattern.Error())
			}()
			obj := core.NewObjectFromMap(core.ValMap{"a": core.True}, ctx)

			NewSet(ctx, core.NewWrappedValueList(obj), config)
		}()
	})
}

func TestPersistLoadSet(t *testing.T) {
	setup := func() (*core.Context, core.SerializedValueStorage) {
		ctx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
		fls := fs_ns.NewMemFilesystem(MAX_MEM_FS_SIZE)
		kv := utils.Must(filekv.OpenSingleFileKV(filekv.KvStoreConfig{
			Filesystem: fls,
			Path:       "/kv",
		}))
		storage := filekv.NewSerializedValueStorage(kv, "ldb://main/")
		return ctx, storage
	}

	t.Run("empty", func(t *testing.T) {
		ctx, storage := setup()

		pattern := NewSetPattern(SetConfig{
			Uniqueness: containers_common.UniquenessConstraint{
				Type: containers_common.UniqueRepr,
			},
		}, core.CallBasedPatternReprMixin{})
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

		loadedSet, err := loadSet(ctx, core.InstanceLoadArgs{
			Key: "/set", Storage: storage, Pattern: pattern,
		})
		if !assert.NoError(t, err) {
			return
		}

		assert.NotNil(t, loadedSet)
	})

	t.Run("unique repr: single element", func(t *testing.T) {
		ctx, storage := setup()

		pattern := NewSetPattern(SetConfig{
			Uniqueness: containers_common.UniquenessConstraint{
				Type: containers_common.UniqueRepr,
			},
		}, core.CallBasedPatternReprMixin{})
		set := NewSetWithConfig(ctx, nil, pattern.config)

		set.Add(ctx, core.Int(1))

		//persist
		{
			persistSet(ctx, set, "/set", storage)

			serialized, ok := storage.GetSerialized(ctx, "/set")
			if !assert.True(t, ok) {
				return
			}
			assert.Equal(t, `[{"int__value":"1"}]`, serialized)
		}

		loadedSet, err := loadSet(ctx, core.InstanceLoadArgs{
			Key: "/set", Storage: storage, Pattern: pattern,
		})
		if !assert.NoError(t, err) {
			return
		}

		assert.True(t, bool(loadedSet.(*Set).Has(ctx, core.Int(1))))
	})

	t.Run("unique repr: two elements", func(t *testing.T) {
		ctx, storage := setup()

		pattern := NewSetPattern(SetConfig{
			Uniqueness: containers_common.UniquenessConstraint{
				Type: containers_common.UniqueRepr,
			},
		}, core.CallBasedPatternReprMixin{})
		set := NewSetWithConfig(ctx, nil, pattern.config)

		set.Add(ctx, core.Int(1))
		set.Add(ctx, core.Int(2))

		//persist
		{
			persistSet(ctx, set, "/set", storage)

			serialized, ok := storage.GetSerialized(ctx, "/set")
			if !assert.True(t, ok) {
				return
			}
			assert.Regexp(t, `(\[{"int__value":"1"},{"int__value":"2"}]|\[{"int__value":"2"},{"int__value":"1"}])`, serialized)
		}

		loadedSet, err := loadSet(ctx, core.InstanceLoadArgs{
			Key: "/set", Storage: storage, Pattern: pattern,
		})
		if !assert.NoError(t, err) {
			return
		}

		assert.True(t, bool(loadedSet.(*Set).Has(ctx, core.Int(1))))
		assert.True(t, bool(loadedSet.(*Set).Has(ctx, core.Int(2))))
	})

	t.Run("unique repr: element with non-unique repr", func(t *testing.T) {
		ctx, storage := setup()

		pattern := NewSetPattern(SetConfig{
			Uniqueness: containers_common.UniquenessConstraint{
				Type: containers_common.UniqueRepr,
			},
		}, core.CallBasedPatternReprMixin{})

		//a mutable object is not considered to have a unique representation.

		storage.SetSerialized(ctx, "/set", `[{"object__value":{}}]`)
		set, err := loadSet(ctx, core.InstanceLoadArgs{
			Key: "/set", Storage: storage, Pattern: pattern,
		})
		if !assert.ErrorIs(t, err, core.ErrReprOfMutableValueCanChange) {
			return
		}

		assert.Nil(t, set)
	})

	t.Run("unique property value: element with missing property", func(t *testing.T) {
		ctx, storage := setup()

		pattern := NewSetPattern(SetConfig{
			Uniqueness: containers_common.UniquenessConstraint{
				Type:         containers_common.UniquePropertyValue,
				PropertyName: "id",
			},
		}, core.CallBasedPatternReprMixin{})

		storage.SetSerialized(ctx, "/set", `[{"object__value":{}}]`)
		set, err := loadSet(ctx, core.InstanceLoadArgs{
			Key: "/set", Storage: storage, Pattern: pattern,
		})
		if !assert.ErrorIs(t, err, containers_common.ErrFailedGetUniqueKeyPropMissing) {
			return
		}

		assert.Nil(t, set)
	})

	t.Run("unique property value: one element", func(t *testing.T) {
		ctx, storage := setup()

		pattern := NewSetPattern(SetConfig{
			Uniqueness: containers_common.UniquenessConstraint{
				Type:         containers_common.UniquePropertyValue,
				PropertyName: "id",
			},
		}, core.CallBasedPatternReprMixin{})
		set := NewSetWithConfig(ctx, nil, pattern.config)

		set.Add(ctx, core.NewObjectFromMap(core.ValMap{"id": core.Str("a")}, ctx))

		//persist
		{
			persistSet(ctx, set, "/set", storage)

			serialized, ok := storage.GetSerialized(ctx, "/set")
			if !assert.True(t, ok) {
				return
			}
			assert.Equal(t, `[{"object__value":{"id":"a"}}]`, serialized)
		}

		loadedSet, err := loadSet(ctx, core.InstanceLoadArgs{
			Key: "/set", Storage: storage, Pattern: pattern,
		})
		if !assert.NoError(t, err) {
			return
		}

		assert.NotNil(t, loadedSet)
	})

	t.Run("unique property value: two elements", func(t *testing.T) {
		ctx, storage := setup()

		pattern := NewSetPattern(SetConfig{
			Uniqueness: containers_common.UniquenessConstraint{
				Type:         containers_common.UniquePropertyValue,
				PropertyName: "id",
			},
		}, core.CallBasedPatternReprMixin{})
		set := NewSetWithConfig(ctx, nil, pattern.config)

		set.Add(ctx, core.NewObjectFromMap(core.ValMap{"id": core.Str("a")}, ctx))
		set.Add(ctx, core.NewObjectFromMap(core.ValMap{"id": core.Str("b")}, ctx))

		//persist
		{
			persistSet(ctx, set, "/set", storage)

			serialized, ok := storage.GetSerialized(ctx, "/set")
			if !assert.True(t, ok) {
				return
			}
			if strings.Index(serialized, `"a"`) < strings.Index(serialized, `"b"`) {
				assert.Equal(t, `[{"object__value":{"id":"a"}},{"object__value":{"id":"b"}}]`, serialized)
			} else {
				assert.Equal(t, `[{"object__value":{"id":"b"}},{"object__value":{"id":"a"}}]`, serialized)
			}
		}

		loadedSet, err := loadSet(ctx, core.InstanceLoadArgs{
			Key: "/set", Storage: storage, Pattern: pattern,
		})
		if !assert.NoError(t, err) {
			return
		}

		assert.NotNil(t, loadedSet)
	})

	t.Run("unique property value: two elements with same unique prop", func(t *testing.T) {
		ctx, storage := setup()

		pattern := NewSetPattern(SetConfig{
			Uniqueness: containers_common.UniquenessConstraint{
				Type:         containers_common.UniquePropertyValue,
				PropertyName: "id",
			},
		}, core.CallBasedPatternReprMixin{})

		storage.SetSerialized(ctx, "/set", `[{"object__value":{"id": "a"}}, {"object__value":{"id": "a"}}]`)
		set, err := loadSet(ctx, core.InstanceLoadArgs{
			Key: "/set", Storage: storage, Pattern: pattern,
		})
		if !assert.ErrorIs(t, err, ErrValueWithSameKeyAlreadyPresent) {
			return
		}

		assert.Nil(t, set)
	})

}

func TestSetAddRemove(t *testing.T) {

	setup := func() (*core.Context, core.SerializedValueStorage) {
		ctx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
		fls := fs_ns.NewMemFilesystem(MAX_MEM_FS_SIZE)
		kv := utils.Must(filekv.OpenSingleFileKV(filekv.KvStoreConfig{
			Filesystem: fls,
			Path:       "/kv",
		}))
		storage := filekv.NewSerializedValueStorage(kv, "ldb://main/")
		return ctx, storage
	}

	t.Run("add different elements during separate transactions", func(t *testing.T) {
		ctx1 := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
		defer ctx1.Cancel()
		core.StartNewTransaction(ctx1)

		ctx2 := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
		defer ctx2.Cancel()
		core.StartNewTransaction(ctx2)

		set := NewSetWithConfig(ctx1, nil, SetConfig{
			Element: core.INT_PATTERN,
			Uniqueness: containers_common.UniquenessConstraint{
				Type: containers_common.UniqueRepr,
			},
		})
		set.Share(ctx1.GetClosestState())

		set.Add(ctx1, core.Int(1))
		//check that 2 is only added from ctx1's POV
		assert.True(t, bool(set.Has(ctx1, core.Int(1))))
		assert.False(t, bool(set.Has(ctx2, core.Int(1))))

		set.Add(ctx2, core.Int(2))
		//check that 2 is only added from ctx2's POV
		assert.True(t, bool(set.Has(ctx2, core.Int(2))))
		assert.False(t, bool(set.Has(ctx1, core.Int(2))))

		//check that 1 is still only added from ctx1's POV
		assert.True(t, bool(set.Has(ctx1, core.Int(1))))
		assert.False(t, bool(set.Has(ctx2, core.Int(1))))
	})

	t.Run("add then remove different elements during separate transactions", func(t *testing.T) {
		ctx1 := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
		defer ctx1.Cancel()
		core.StartNewTransaction(ctx1)

		ctx2 := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
		defer ctx2.Cancel()
		core.StartNewTransaction(ctx2)

		set := NewSetWithConfig(ctx1, nil, SetConfig{
			Element: core.INT_PATTERN,
			Uniqueness: containers_common.UniquenessConstraint{
				Type: containers_common.UniqueRepr,
			},
		})

		set.Share(ctx1.GetClosestState())

		set.Add(ctx1, core.Int(1))
		//check that 2 is only added from ctx1's POV
		assert.True(t, bool(set.Has(ctx1, core.Int(1))))
		_, found := set.Get(ctx1, core.Str("1"))
		assert.True(t, bool(found))
		assert.False(t, bool(set.Has(ctx2, core.Int(1))))
		_, found = set.Get(ctx2, core.Str("1"))
		assert.False(t, bool(found))

		set.Add(ctx2, core.Int(2))
		//check that 2 is only added from ctx2's POV
		assert.True(t, bool(set.Has(ctx2, core.Int(2))))
		_, found = set.Get(ctx2, core.Str("2"))
		assert.True(t, bool(found))
		assert.False(t, bool(set.Has(ctx1, core.Int(2))))
		_, found = set.Get(ctx1, core.Str("2"))
		assert.False(t, bool(found))

		//check that 1 is still only added from ctx1's POV
		assert.True(t, bool(set.Has(ctx1, core.Int(1))))
		_, found = set.Get(ctx1, core.Str("1"))
		assert.True(t, bool(found))
		assert.False(t, bool(set.Has(ctx2, core.Int(1))))
		_, found = set.Get(ctx2, core.Str("1"))
		assert.False(t, bool(found))

		set.Remove(ctx1, core.Int(1))
		assert.False(t, bool(set.Has(ctx1, core.Int(1))))
		_, found = set.Get(ctx1, core.Str("1"))
		assert.False(t, bool(found))
		assert.False(t, bool(set.Has(ctx2, core.Int(1))))
		_, found = set.Get(ctx2, core.Str("1"))
		assert.False(t, bool(found))

		set.Remove(ctx2, core.Int(2))
		assert.False(t, bool(set.Has(ctx2, core.Int(2))))
		_, found = set.Get(ctx2, core.Str("2"))
		assert.False(t, bool(found))
		assert.False(t, bool(set.Has(ctx1, core.Int(2))))
		_, found = set.Get(ctx1, core.Str("2"))
		assert.False(t, bool(found))

		assert.False(t, bool(set.Has(ctx1, core.Int(1))))
		_, found = set.Get(ctx1, core.Str("1"))
		assert.False(t, bool(found))
		assert.False(t, bool(set.Has(ctx2, core.Int(1))))
		_, found = set.Get(ctx2, core.Str("1"))
		assert.False(t, bool(found))
	})

	t.Run("remove different elements during separate transactions", func(t *testing.T) {
		ctx0 := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
		defer ctx0.Cancel()

		ctx1 := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
		defer ctx1.Cancel()
		core.StartNewTransaction(ctx1)

		ctx2 := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
		defer ctx2.Cancel()
		core.StartNewTransaction(ctx2)

		set := NewSetWithConfig(ctx0, core.NewWrappedValueList(core.Int(1), core.Int(2)), SetConfig{
			Element: core.INT_PATTERN,
			Uniqueness: containers_common.UniquenessConstraint{
				Type: containers_common.UniqueRepr,
			},
		})

		set.Share(ctx1.GetClosestState())

		set.Remove(ctx1, core.Int(1))
		//check that 2 is only removed from ctx1's POV
		assert.False(t, bool(set.Has(ctx1, core.Int(1))))
		assert.True(t, bool(set.Has(ctx2, core.Int(1))))

		set.Remove(ctx2, core.Int(2))
		//check that 2 is only removed from ctx2's POV
		assert.False(t, bool(set.Has(ctx2, core.Int(2))))
		assert.True(t, bool(set.Has(ctx1, core.Int(2))))

		//check that 1 is still removed added from ctx1's POV
		assert.False(t, bool(set.Has(ctx1, core.Int(1))))
		assert.True(t, bool(set.Has(ctx2, core.Int(1))))
	})

	t.Run("Set should be persisted during call to .Add", func(t *testing.T) {
		ctx, storage := setup()
		defer ctx.Cancel()

		pattern := NewSetPattern(SetConfig{
			Uniqueness: containers_common.UniquenessConstraint{
				Type: containers_common.UniqueRepr,
			},
		}, core.CallBasedPatternReprMixin{})

		storage.SetSerialized(ctx, "/set", `[]`)
		set, err := loadSet(ctx, core.InstanceLoadArgs{
			Key: "/set", Storage: storage, Pattern: pattern,
		})

		set.(*Set).Share(ctx.GetClosestState())

		if !assert.NoError(t, err) {
			return
		}

		set.(*Set).Add(ctx, core.Int(1))

		//check that the Set is persisted

		persisted, err := loadSet(ctx, core.InstanceLoadArgs{
			Key: "/set", Storage: storage, Pattern: pattern,
		})

		if !assert.NoError(t, err) {
			return
		}

		assert.NotSame(t, persisted, set) //future-proofing the test

		vals := core.IterateAllValuesOnly(ctx, set.(*Set).Iterator(ctx, core.IteratorConfiguration{}))
		if !assert.Len(t, vals, 1) {
			return
		}

		val := vals[0]

		assert.Equal(t, core.Int(1), val)
	})

	t.Run("Set should be persisted at end of transaction if .Add was called transactionnaly", func(t *testing.T) {

		ctx, storage := setup()
		tx := core.StartNewTransaction(ctx)

		pattern := NewSetPattern(SetConfig{
			Uniqueness: containers_common.UniquenessConstraint{
				Type: containers_common.UniqueRepr,
			},
		}, core.CallBasedPatternReprMixin{})

		storage.SetSerialized(ctx, "/set", `[]`)
		set, err := loadSet(ctx, core.InstanceLoadArgs{
			Key: "/set", Storage: storage, Pattern: pattern,
		})

		set.(*Set).Share(ctx.GetClosestState())

		if !assert.NoError(t, err) {
			return
		}

		set.(*Set).Add(ctx, core.Int(1))

		//check that the Set is not persised

		persisted, err := loadSet(ctx, core.InstanceLoadArgs{
			Key: "/set", Storage: storage, Pattern: pattern,
		})

		if !assert.NoError(t, err) {
			return
		}

		assert.NotSame(t, persisted, set) //future-proofing the test
		assert.False(t, bool(persisted.(*Set).Has(ctx, core.Int(1))))

		assert.NoError(t, tx.Commit(ctx))

		//check that the Set is not persised

		persisted, err = loadSet(ctx, core.InstanceLoadArgs{
			Key: "/set", Storage: storage, Pattern: pattern,
		})

		if !assert.NoError(t, err) {
			return
		}

		assert.NotSame(t, persisted, set) //future-proofing the test
		assert.True(t, bool(persisted.(*Set).Has(ctx, core.Int(1))))
	})

	t.Run("transient Set should be updated if .Add was called transactionnaly", func(t *testing.T) {
		ctx1, storage := setup()
		tx1 := core.StartNewTransaction(ctx1)

		ctx2 := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
		core.StartNewTransaction(ctx2)

		pattern := NewSetPattern(SetConfig{
			Uniqueness: containers_common.UniquenessConstraint{
				Type: containers_common.UniqueRepr,
			},
		}, core.CallBasedPatternReprMixin{})

		storage.SetSerialized(ctx1, "/set", `[]`)
		val, err := loadSet(ctx1, core.InstanceLoadArgs{
			Key: "/set", Storage: storage, Pattern: pattern,
		})

		if !assert.NoError(t, err) {
			return
		}

		set := val.(*Set)
		set.Share(ctx1.GetClosestState())

		set.Add(ctx1, core.Int(1))

		//check that the Set is not updated from the other ctx's POV
		assert.False(t, bool(set.Has(ctx2, core.Int(1))))

		utils.PanicIfErr(tx1.Commit(ctx1))

		//check that the Set is updated from the other ctx's POV
		assert.True(t, bool(set.Has(ctx2, core.Int(1))))
	})

	t.Run("Set should be persisted during call to .Remove", func(t *testing.T) {
		ctx, storage := setup()

		pattern := NewSetPattern(SetConfig{
			Uniqueness: containers_common.UniquenessConstraint{
				Type: containers_common.UniqueRepr,
			},
		}, core.CallBasedPatternReprMixin{})

		storage.SetSerialized(ctx, "/set", `[{"int__value":"1"}]`)
		val, err := loadSet(ctx, core.InstanceLoadArgs{
			Key: "/set", Storage: storage, Pattern: pattern,
		})

		if !assert.NoError(t, err) {
			return
		}

		set := val.(*Set)
		set.Share(ctx.GetClosestState())

		set.Remove(ctx, core.Int(1))

		//check that the Set is persised

		persisted, err := loadSet(ctx, core.InstanceLoadArgs{
			Key: "/set", Storage: storage, Pattern: pattern,
		})

		if !assert.NoError(t, err) {
			return
		}

		assert.NotSame(t, persisted, set) //future-proofing the test

		vals := core.IterateAllValuesOnly(ctx, set.Iterator(ctx, core.IteratorConfiguration{}))
		assert.Len(t, vals, 0)
	})

	t.Run("Set should be persisted at end of transaction if .Remove was called transactionnaly", func(t *testing.T) {

		ctx, storage := setup()
		tx := core.StartNewTransaction(ctx)

		pattern := NewSetPattern(SetConfig{
			Uniqueness: containers_common.UniquenessConstraint{
				Type: containers_common.UniqueRepr,
			},
		}, core.CallBasedPatternReprMixin{})

		storage.SetSerialized(ctx, "/set", `[{"int__value":"1"}]`)
		val, err := loadSet(ctx, core.InstanceLoadArgs{
			Key: "/set", Storage: storage, Pattern: pattern,
		})

		if !assert.NoError(t, err) {
			return
		}

		set := val.(*Set)
		set.Share(ctx.GetClosestState())

		set.Remove(ctx, core.Int(1))

		//check that the Set is not persised

		persisted, err := loadSet(ctx, core.InstanceLoadArgs{
			Key: "/set", Storage: storage, Pattern: pattern,
		})

		if !assert.NoError(t, err) {
			return
		}

		assert.NotSame(t, persisted, set) //future-proofing the test
		assert.True(t, bool(persisted.(*Set).Has(ctx, core.Int(1))))

		assert.NoError(t, tx.Commit(ctx))

		//check that the Set is not persised

		persisted, err = loadSet(ctx, core.InstanceLoadArgs{
			Key: "/set", Storage: storage, Pattern: pattern,
		})

		if !assert.NoError(t, err) {
			return
		}

		assert.NotSame(t, persisted, set) //future-proofing the test
		assert.False(t, bool(persisted.(*Set).Has(ctx, core.Int(1))))
	})
	t.Run("url holder with no URL should get one if Set is persistent", func(t *testing.T) {
		ctx, storage := setup()

		pattern := NewSetPattern(SetConfig{
			Uniqueness: containers_common.UniquenessConstraint{
				Type: containers_common.UniqueURL,
			},
		}, core.CallBasedPatternReprMixin{})

		storage.SetSerialized(ctx, "/set", `[]`)
		val, err := loadSet(ctx, core.InstanceLoadArgs{
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

}

func TestInteractWithElementsOfLoadedSet(t *testing.T) {
	setup := func() (*core.Context, core.SerializedValueStorage) {
		ctx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
		fls := fs_ns.NewMemFilesystem(MAX_MEM_FS_SIZE)
		kv := utils.Must(filekv.OpenSingleFileKV(filekv.KvStoreConfig{
			Filesystem: fls,
			Path:       "/kv",
		}))
		storage := filekv.NewSerializedValueStorage(kv, "ldb://main/")
		return ctx, storage
	}

	t.Run("adding a simple value property to an element should trigger a persistence", func(t *testing.T) {
		ctx, storage := setup()

		pattern := NewSetPattern(SetConfig{
			Uniqueness: containers_common.UniquenessConstraint{
				Type: containers_common.UniqueURL,
			},
		}, core.CallBasedPatternReprMixin{})

		storage.SetSerialized(ctx, "/set", `[]`)
		set, err := loadSet(ctx, core.InstanceLoadArgs{
			Key: "/set", Storage: storage, Pattern: pattern,
		})

		if !assert.NoError(t, err) {
			return
		}

		newElem := core.NewObjectFromMap(core.ValMap{}, ctx)
		set.(*Set).Add(ctx, newElem)

		url, _ := newElem.URL()

		//load again

		loadedSet, err := loadSet(ctx, core.InstanceLoadArgs{
			Key: "/set", Storage: storage, Pattern: pattern,
		})

		if !assert.NoError(t, err) {
			return
		}

		assert.NotSame(t, set, loadedSet) //future-proofing the test

		elem, _ := loadedSet.(*Set).Get(ctx, core.Str(url.UnderlyingString()))
		obj := elem.(*core.Object)
		if !assert.NoError(t, obj.SetProp(ctx, "prop", core.Int(1))) {
			return
		}

		//load again

		loadedSet, err = loadSet(ctx, core.InstanceLoadArgs{
			Key: "/set", Storage: storage, Pattern: pattern,
		})
		if !assert.NoError(t, err) {
			return
		}

		loadedElem, _ := loadedSet.(*Set).Get(ctx, core.Str(url.UnderlyingString()))
		loadedObj := loadedElem.(*core.Object)

		if !assert.Equal(t, []string{"prop"}, loadedObj.PropertyNames(ctx)) {
			return
		}

		assert.Equal(t, core.Int(1), loadedObj.Prop(ctx, "prop"))
	})
}

func TestSetMigrate(t *testing.T) {

	config := SetConfig{
		Element:    core.SERIALIZABLE_PATTERN,
		Uniqueness: containers_common.UniquenessConstraint{Type: containers_common.UniqueRepr},
	}

	t.Run("delete Set: / key", func(t *testing.T) {
		ctx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
		set := NewSetWithConfig(ctx, nil, config)
		val, err := set.Migrate(ctx, "/", &core.InstanceMigrationArgs{
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
		ctx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
		set := NewSetWithConfig(ctx, nil, config)
		val, err := set.Migrate(ctx, "/x", &core.InstanceMigrationArgs{
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
		ctx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
		set := NewSetWithConfig(ctx, core.NewWrappedValueList(core.Int(0)), config)
		val, err := set.Migrate(ctx, "/", &core.InstanceMigrationArgs{
			NextPattern: nil,
			MigrationHandlers: core.MigrationOpHandlers{
				Deletions: map[core.PathPattern]*core.MigrationOpHandler{
					"/0": nil,
				},
			},
		})

		if !assert.NoError(t, err) {
			return
		}
		assert.Same(t, set, val)
		assert.Equal(t, map[string]core.Serializable{}, set.elements)
	})

	t.Run("delete all elements", func(t *testing.T) {
		ctx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
		set := NewSetWithConfig(ctx, core.NewWrappedValueList(core.Int(0), core.Int(1)), config)
		val, err := set.Migrate(ctx, "/", &core.InstanceMigrationArgs{
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
		assert.Equal(t, map[string]core.Serializable{}, set.elements)
	})

	t.Run("delete inexisting element", func(t *testing.T) {
		ctx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
		set := NewSetWithConfig(ctx, core.NewWrappedValueList(core.Int(0)), config)
		val, err := set.Migrate(ctx, "/", &core.InstanceMigrationArgs{
			NextPattern: nil,
			MigrationHandlers: core.MigrationOpHandlers{
				Deletions: map[core.PathPattern]*core.MigrationOpHandler{
					"/1": nil,
				},
			},
		})

		if !assert.Equal(t, err, commonfmt.FmtValueAtPathDoesNotExist("/1")) {
			return
		}
		assert.Nil(t, val)
	})

	t.Run("delete property of element", func(t *testing.T) {
		ctx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
		elements := core.NewWrappedValueList(core.NewRecordFromMap(core.ValMap{"b": core.Int(0)}))
		set := NewSetWithConfig(ctx, elements, config)
		val, err := set.Migrate(ctx, "/", &core.InstanceMigrationArgs{
			NextPattern: nil,
			MigrationHandlers: core.MigrationOpHandlers{
				Deletions: map[core.PathPattern]*core.MigrationOpHandler{
					"/#{\"b\":0}/b": nil,
				},
			},
		})

		if !assert.NoError(t, err) {
			return
		}
		assert.Same(t, set, val)
		expectedElement := core.NewRecordFromMap(core.ValMap{})
		assert.Equal(t, map[string]core.Serializable{"#{}": expectedElement}, set.elements)
	})

	t.Run("replace Set: / key", func(t *testing.T) {
		ctx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
		set := NewSetWithConfig(ctx, core.NewWrappedValueList(), config)
		replacement := core.NewWrappedValueList()

		val, err := set.Migrate(ctx, "/", &core.InstanceMigrationArgs{
			NextPattern: NewSetPattern(config, core.CallBasedPatternReprMixin{}),
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
		ctx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
		set := NewSetWithConfig(ctx, core.NewWrappedValueList(), config)
		replacement := core.NewWrappedValueList()

		val, err := set.Migrate(ctx, "/x", &core.InstanceMigrationArgs{
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
		ctx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
		elements := core.NewWrappedValueList(
			core.NewRecordFromMap(core.ValMap{"a": core.Int(1), "b": core.Int(1)}),
			core.NewRecordFromMap(core.ValMap{"a": core.Int(2), "b": core.Int(2)}),
		)

		set := NewSetWithConfig(ctx, elements, config)
		val, err := set.Migrate(ctx, "/", &core.InstanceMigrationArgs{
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
		assert.Equal(t, map[string]core.Serializable{
			`#{"a":1,"b":3}`: core.NewRecordFromMap(core.ValMap{"a": core.Int(1), "b": core.Int(3)}),
			`#{"a":2,"b":3}`: core.NewRecordFromMap(core.ValMap{"a": core.Int(2), "b": core.Int(3)}),
		}, set.elements)
	})

	t.Run("element inclusion should panic", func(t *testing.T) {
		ctx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)

		set := NewSetWithConfig(ctx, nil, config)

		assert.PanicsWithError(t, core.ErrUnreachable.Error(), func() {
			set.Migrate(ctx, "/", &core.InstanceMigrationArgs{
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
