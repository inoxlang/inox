package containers

import (
	"io"
	"strings"
	"testing"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/filekv"
	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/stretchr/testify/assert"

	coll_symbolic "github.com/inoxlang/inox/internal/globals/containers/symbolic"
)

func TestNewSet(t *testing.T) {

	t.Run("no elements", func(t *testing.T) {
		ctx := core.NewContexWithEmptyState(core.ContextConfig{}, io.Discard)
		set := NewSet(ctx, core.NewWrappedValueList())

		assert.Equal(t, SetConfig{
			Uniqueness: UniquenessConstraint{
				Type: UniqueRepr,
			},
			Element: core.SERIALIZABLE_PATTERN,
		}, set.config)
	})

	t.Run("single element", func(t *testing.T) {
		ctx := core.NewContexWithEmptyState(core.ContextConfig{}, io.Discard)
		set := NewSet(ctx, core.NewWrappedValueList(core.Int(1)))

		assert.Equal(t, SetConfig{
			Uniqueness: UniquenessConstraint{
				Type: UniqueRepr,
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
			Uniqueness: UniquenessConstraint{
				Type: UniqueURL,
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
				assert.ErrorContains(t, recover().(error), ErrFailedGetUniqueKeyNoURL.Error())
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
			Uniqueness: UniquenessConstraint{
				Type:         UniquePropertyValue,
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
				assert.ErrorContains(t, recover().(error), ErrFailedGetUniqueKeyNoProps.Error())
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
			Uniqueness: UniquenessConstraint{
				Type: UniqueRepr,
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
		kv := utils.Must(filekv.OpenSingleFileKV(filekv.KvStoreConfig{
			InMemory: true,
		}))
		storage := filekv.NewSerializedValueStorage(kv, "ldb://main/")
		return ctx, storage
	}

	t.Run("empty", func(t *testing.T) {
		ctx, storage := setup()

		pattern := NewSetPattern(SetConfig{
			Uniqueness: UniquenessConstraint{
				Type: UniqueRepr,
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
			Uniqueness: UniquenessConstraint{
				Type: UniqueRepr,
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
			Uniqueness: UniquenessConstraint{
				Type: UniqueRepr,
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
			Uniqueness: UniquenessConstraint{
				Type: UniqueRepr,
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
			Uniqueness: UniquenessConstraint{
				Type:         UniquePropertyValue,
				PropertyName: "id",
			},
		}, core.CallBasedPatternReprMixin{})

		storage.SetSerialized(ctx, "/set", `[{"object__value":{}}]`)
		set, err := loadSet(ctx, core.InstanceLoadArgs{
			Key: "/set", Storage: storage, Pattern: pattern,
		})
		if !assert.ErrorIs(t, err, ErrFailedGetUniqueKeyPropMissing) {
			return
		}

		assert.Nil(t, set)
	})

	t.Run("unique property value: one element", func(t *testing.T) {
		ctx, storage := setup()

		pattern := NewSetPattern(SetConfig{
			Uniqueness: UniquenessConstraint{
				Type:         UniquePropertyValue,
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
			Uniqueness: UniquenessConstraint{
				Type:         UniquePropertyValue,
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
			Uniqueness: UniquenessConstraint{
				Type:         UniquePropertyValue,
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

func TestSetAddToPersistedSet(t *testing.T) {

	setup := func() (*core.Context, core.SerializedValueStorage) {
		ctx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
		kv := utils.Must(filekv.OpenSingleFileKV(filekv.KvStoreConfig{
			InMemory: true,
		}))
		storage := filekv.NewSerializedValueStorage(kv, "ldb://main/")
		return ctx, storage
	}

	t.Run("representation uniqueness: Set should be persisted during call to .Add", func(t *testing.T) {
		ctx, storage := setup()

		pattern := NewSetPattern(SetConfig{
			Uniqueness: UniquenessConstraint{
				Type: UniqueRepr,
			},
		}, core.CallBasedPatternReprMixin{})

		storage.SetSerialized(ctx, "/set", `[]`)
		set, err := loadSet(ctx, core.InstanceLoadArgs{
			Key: "/set", Storage: storage, Pattern: pattern,
		})

		if !assert.NoError(t, err) {
			return
		}

		set.(*Set).Add(ctx, core.Int(1))

		//check that the Set is persised

		persisted, err := loadSet(ctx, core.InstanceLoadArgs{
			Key: "/set", Storage: storage, Pattern: pattern,
		})

		if !assert.NoError(t, err) {
			return
		}

		//future-proofing the test
		assert.NotSame(t, persisted, set)

		vals := core.IterateAllValuesOnly(ctx, set.(*Set).Iterator(ctx, core.IteratorConfiguration{}))
		if !assert.Len(t, vals, 1) {
			return
		}

		val := vals[0]

		assert.Equal(t, core.Int(1), val)
	})

	t.Run("url holder with no URL should get one", func(t *testing.T) {
		ctx, storage := setup()

		pattern := NewSetPattern(SetConfig{
			Uniqueness: UniquenessConstraint{
				Type: UniqueURL,
			},
		}, core.CallBasedPatternReprMixin{})

		storage.SetSerialized(ctx, "/set", `[]`)
		set, err := loadSet(ctx, core.InstanceLoadArgs{
			Key: "/set", Storage: storage, Pattern: pattern,
		})

		if !assert.NoError(t, err) {
			return
		}

		obj := core.NewObjectFromMap(core.ValMap{}, ctx)
		set.(*Set).Add(ctx, obj)

		url, ok := obj.URL()
		if !assert.True(t, ok) {
			return
		}

		assert.Regexp(t, "ldb://main/.*", string(url))
	})

}

func TestSetRemoveFromPersistedSet(t *testing.T) {

	setup := func() (*core.Context, core.SerializedValueStorage) {
		ctx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
		kv := utils.Must(filekv.OpenSingleFileKV(filekv.KvStoreConfig{
			InMemory: true,
		}))
		storage := filekv.NewSerializedValueStorage(kv, "ldb://main/")
		return ctx, storage
	}

	t.Run("representation uniqueness", func(t *testing.T) {
		ctx, storage := setup()

		pattern := NewSetPattern(SetConfig{
			Uniqueness: UniquenessConstraint{
				Type: UniqueRepr,
			},
		}, core.CallBasedPatternReprMixin{})

		storage.SetSerialized(ctx, "/set", `[]`)
		set, err := loadSet(ctx, core.InstanceLoadArgs{
			Key: "/set", Storage: storage, Pattern: pattern,
		})

		if !assert.NoError(t, err) {
			return
		}

		set.(*Set).Add(ctx, core.Int(1))
		set.(*Set).Remove(ctx, core.Int(1))

		//check that the Set is persised

		persisted, err := loadSet(ctx, core.InstanceLoadArgs{
			Key: "/set", Storage: storage, Pattern: pattern,
		})

		if !assert.NoError(t, err) {
			return
		}

		assert.False(t, bool(persisted.(*Set).Has(ctx, core.Int(1))))
	})

}
