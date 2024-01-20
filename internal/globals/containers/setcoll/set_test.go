package setcoll

import (
	"io"
	"testing"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/containers/common"
	coll_symbolic "github.com/inoxlang/inox/internal/globals/containers/symbolic"
	"github.com/inoxlang/inox/internal/parse"
	"github.com/stretchr/testify/assert"
)

func TestNewSet(t *testing.T) {

	t.Run("no elements", func(t *testing.T) {
		ctx := core.NewContexWithEmptyState(core.ContextConfig{}, io.Discard)
		set := NewSet(ctx, core.NewWrappedValueList(), nil)

		assert.Equal(t, SetConfig{
			Uniqueness: common.UniquenessConstraint{
				Type: common.UniqueRepr,
			},
			Element: core.SERIALIZABLE_PATTERN,
		}, set.config)
	})

	t.Run("single element", func(t *testing.T) {
		ctx := core.NewContexWithEmptyState(core.ContextConfig{}, io.Discard)
		set := NewSet(ctx, core.NewWrappedValueList(core.Int(1)), nil)

		assert.Equal(t, SetConfig{
			Uniqueness: common.UniquenessConstraint{
				Type: common.UniqueRepr,
			},
			Element: core.SERIALIZABLE_PATTERN,
		}, set.config)
	})

	t.Run("element with no representation yet", func(t *testing.T) {
		ctx := core.NewContexWithEmptyState(core.ContextConfig{}, io.Discard)

		node := core.AstNode{Node: parse.MustParseChunk("")}

		func() {
			defer func() {
				assert.ErrorContains(t, recover().(error), "not implemented yet")
			}()
			NewSet(ctx, core.NewWrappedValueList(node), nil)
		}()
	})

	t.Run("element with representation should be immutable ", func(t *testing.T) {
		ctx := core.NewContexWithEmptyState(core.ContextConfig{}, io.Discard)

		obj := core.NewObjectFromMap(core.ValMap{}, ctx)

		func() {
			defer func() {
				assert.ErrorContains(t, recover().(error), core.ErrReprOfMutableValueCanChange.Error())
			}()
			NewSet(ctx, core.NewWrappedValueList(obj), nil)
		}()
	})

	t.Run("uniqueness of property's value", func(t *testing.T) {
		ctx := core.NewContexWithEmptyState(core.ContextConfig{}, io.Discard)

		config := core.NewObjectFromMap(core.ValMap{
			coll_symbolic.SET_CONFIG_UNIQUE_PROP_KEY: core.PropertyName("id"),
		}, ctx)

		set := NewSet(ctx, core.NewWrappedValueList(), core.ToOptionalParam(config))

		assert.Equal(t, SetConfig{
			Uniqueness: common.UniquenessConstraint{
				Type:         common.UniquePropertyValue,
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
				assert.ErrorContains(t, recover().(error), common.ErrFailedGetUniqueKeyNoProps.Error())
			}()
			NewSet(ctx, core.NewWrappedValueList(core.Int(1)), core.ToOptionalParam(config))
		}()
	})

	t.Run("element pattern", func(t *testing.T) {
		ctx := core.NewContexWithEmptyState(core.ContextConfig{}, io.Discard)

		elementPattern := core.NewInexactObjectPattern([]core.ObjectPatternEntry{
			{Name: "a", Pattern: core.INT_PATTERN},
		})

		config := core.NewObjectFromMap(core.ValMap{
			coll_symbolic.SET_CONFIG_ELEMENT_PATTERN_PROP_KEY: elementPattern,
		}, ctx)

		set := NewSet(ctx, core.NewWrappedValueList(), core.ToOptionalParam(config))

		assert.Equal(t, SetConfig{
			Uniqueness: common.UniquenessConstraint{
				Type: common.UniqueRepr,
			},
			Element: elementPattern,
		}, set.config)
	})

	t.Run("element pattern: element does not match", func(t *testing.T) {
		ctx := core.NewContexWithEmptyState(core.ContextConfig{}, io.Discard)

		elementPattern := core.NewInexactObjectPattern([]core.ObjectPatternEntry{
			{Name: "a", Pattern: core.INT_PATTERN},
		})

		config := core.NewObjectFromMap(core.ValMap{
			coll_symbolic.SET_CONFIG_ELEMENT_PATTERN_PROP_KEY: elementPattern,
		}, ctx)

		func() {
			defer func() {
				assert.ErrorContains(t, recover().(error), ErrValueDoesMatchElementPattern.Error())
			}()
			record := core.NewRecordFromMap(core.ValMap{"a": core.True})

			NewSet(ctx, core.NewWrappedValueList(record), core.ToOptionalParam(config))
		}()
	})
}

func TestUnsharedSetAddRemove(t *testing.T) {
	ctx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)

	t.Run("representation uniqueness", func(t *testing.T) {
		set := NewSetWithConfig(ctx, nil, SetConfig{
			Element: core.INT_PATTERN,
			Uniqueness: common.UniquenessConstraint{
				Type: common.UniqueRepr,
			},
		})

		int1 := core.Int(1)
		set.Add(ctx, int1)
		assert.True(t, bool(set.Has(ctx, int1)))
		assert.False(t, bool(set.Has(ctx, core.Int(2))))

		set.Remove(ctx, int1)
		assert.False(t, bool(set.Has(ctx, int1)))

		//invalid element
		assert.PanicsWithError(t, ErrValueDoesMatchElementPattern.Error(), func() {
			set.Add(ctx, core.True)
		})
	})

	t.Run("representation uniquenesss: element with sensitive data", func(t *testing.T) {
		set := NewSetWithConfig(ctx, nil, SetConfig{
			Element: core.RECORD_PATTERN,
			Uniqueness: common.UniquenessConstraint{
				Type: common.UniqueRepr,
			},
		})

		record := core.NewRecordFromMap(core.ValMap{"password": core.Str("x"), "email-address": core.EmailAddress("a@mail.com")})
		set.Add(ctx, record)

		val, ok := set.Get(ctx, core.Str(`{"email-address":{"emailaddr__value":"a@mail.com"},"password":"x"}`))

		if !assert.True(t, bool(ok)) {
			return
		}

		assert.Same(t, record, val)
	})

	t.Run("property value uniqueness", func(t *testing.T) {
		set := NewSetWithConfig(ctx, nil, SetConfig{
			Element: core.RECORD_PATTERN,
			Uniqueness: common.UniquenessConstraint{
				Type:         common.UniquePropertyValue,
				PropertyName: "x",
			},
		})

		record1 := core.NewRecordFromMap(core.ValMap{"x": core.Int(1)})
		record2 := core.NewRecordFromMap(core.ValMap{"x": core.Int(2)})

		set.Add(ctx, record1)
		assert.True(t, bool(set.Has(ctx, record1)))
		assert.False(t, bool(set.Has(ctx, record2)))

		set.Remove(ctx, record1)
		assert.False(t, bool(set.Has(ctx, record1)))

		//invalid element
		assert.PanicsWithError(t, ErrValueDoesMatchElementPattern.Error(), func() {
			set.Add(ctx, core.True)
		})
	})

	t.Run("adding an element to an unshared URL-based uniqueness Set should cause a panic", func(t *testing.T) {
		set := NewSetWithConfig(ctx, nil, SetConfig{
			Element: core.RECORD_PATTERN,
			Uniqueness: common.UniquenessConstraint{
				PropertyName: "x",
			},
		})

		obj := core.NewObjectFromMapNoInit(core.ValMap{"x": core.Int(1)})

		assert.Panics(t, func() {
			set.Add(ctx, obj)
		})
	})
}
