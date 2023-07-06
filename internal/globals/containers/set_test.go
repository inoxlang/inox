package containers

import (
	"io"
	"testing"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/filekv"
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
		}, set.config)
	})

	t.Run("single element", func(t *testing.T) {
		ctx := core.NewContexWithEmptyState(core.ContextConfig{}, io.Discard)
		set := NewSet(ctx, core.NewWrappedValueList(core.Int(1)))

		assert.Equal(t, SetConfig{
			Uniqueness: UniquenessConstraint{
				Type: UniqueRepr,
			},
		}, set.config)
	})

	t.Run("element with no representation", func(t *testing.T) {
		ctx := core.NewContexWithEmptyState(core.ContextConfig{}, io.Discard)

		obj := core.NewObject()
		obj.SetProp(ctx, "self", obj)

		func() {
			defer func() {
				assert.ErrorContains(t, recover().(error), "failed to get representation")
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
		}, set.config)
	})

	t.Run("url uniqueness: element has no URL", func(t *testing.T) {
		ctx := core.NewContexWithEmptyState(core.ContextConfig{}, io.Discard)

		config := core.NewObjectFromMap(core.ValMap{
			coll_symbolic.SET_CONFIG_UNIQUE_PROP_KEY: core.Identifier("url"),
		}, ctx)

		func() {
			defer func() {
				assert.ErrorContains(t, recover().(error), ErrFailedGetUniqueKeyNoURL.Error())
			}()
			NewSet(ctx, core.NewWrappedValueList(core.Int(1)), config)
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

func TestLoadSet(t *testing.T) {

	setup := func() (*core.Context, core.SerializedValueStorage) {
		ctx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
		kv := utils.Must(filekv.OpenSingleFileKV(filekv.KvStoreConfig{
			InMemory: true,
		}))
		storage := filekv.NewSerializedValueStorage(kv)
		return ctx, storage
	}

	t.Run("empty", func(t *testing.T) {
		ctx, storage := setup()

		pattern := NewSetPattern(SetConfig{
			Uniqueness: UniquenessConstraint{
				Type: UniqueRepr,
			},
		}, core.CallBasedPatternReprMixin{})

		storage.SetSerialized(ctx, "/set", `[]`)
		set, err := loadSet(ctx, "/set", storage, pattern)
		if !assert.NoError(t, err) {
			return
		}

		assert.NotNil(t, set)
	})

	t.Run("unique repr: single element", func(t *testing.T) {
		ctx, storage := setup()

		pattern := NewSetPattern(SetConfig{
			Uniqueness: UniquenessConstraint{
				Type: UniqueRepr,
			},
		}, core.CallBasedPatternReprMixin{})

		storage.SetSerialized(ctx, "/set", `[1]`)
		set, err := loadSet(ctx, "/set", storage, pattern)
		if !assert.NoError(t, err) {
			return
		}

		assert.True(t, bool(set.(*Set).Has(ctx, core.Int(1))))
	})

	t.Run("unique repr: two elements", func(t *testing.T) {
		ctx, storage := setup()

		pattern := NewSetPattern(SetConfig{
			Uniqueness: UniquenessConstraint{
				Type: UniqueRepr,
			},
		}, core.CallBasedPatternReprMixin{})

		storage.SetSerialized(ctx, "/set", `[1, 2]`)
		set, err := loadSet(ctx, "/set", storage, pattern)
		if !assert.NoError(t, err) {
			return
		}

		assert.True(t, bool(set.(*Set).Has(ctx, core.Int(1))))
		assert.True(t, bool(set.(*Set).Has(ctx, core.Int(2))))
	})

	t.Run("unique repr: element with non-unique repr", func(t *testing.T) {
		ctx, storage := setup()

		pattern := NewSetPattern(SetConfig{
			Uniqueness: UniquenessConstraint{
				Type: UniqueRepr,
			},
		}, core.CallBasedPatternReprMixin{})

		//a mutable object is not considered to have a unique representation.

		storage.SetSerialized(ctx, "/set", `[{}]`)
		set, err := loadSet(ctx, "/set", storage, pattern)
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

		storage.SetSerialized(ctx, "/set", `[{}]`)
		set, err := loadSet(ctx, "/set", storage, pattern)
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

		storage.SetSerialized(ctx, "/set", `[{"id": "a"}]`)
		set, err := loadSet(ctx, "/set", storage, pattern)
		if !assert.NoError(t, err) {
			return
		}

		assert.NotNil(t, set)
	})

	t.Run("unique property value: two elements", func(t *testing.T) {
		ctx, storage := setup()

		pattern := NewSetPattern(SetConfig{
			Uniqueness: UniquenessConstraint{
				Type:         UniquePropertyValue,
				PropertyName: "id",
			},
		}, core.CallBasedPatternReprMixin{})

		storage.SetSerialized(ctx, "/set", `[{"id": "a"}, {"id": "b"}]`)
		set, err := loadSet(ctx, "/set", storage, pattern)
		if !assert.NoError(t, err) {
			return
		}

		assert.NotNil(t, set)
	})

	t.Run("unique property value: two elements with same unique prop", func(t *testing.T) {
		ctx, storage := setup()

		pattern := NewSetPattern(SetConfig{
			Uniqueness: UniquenessConstraint{
				Type:         UniquePropertyValue,
				PropertyName: "id",
			},
		}, core.CallBasedPatternReprMixin{})

		storage.SetSerialized(ctx, "/set", `[{"id": "a"}, {"id": "a"}]`)
		set, err := loadSet(ctx, "/set", storage, pattern)
		if !assert.ErrorIs(t, err, ErrValueWithSameKeyAlreadyPresent) {
			return
		}

		assert.Nil(t, set)
	})

}
