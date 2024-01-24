package mapcoll

import (
	"io"
	"testing"

	"github.com/inoxlang/inox/internal/core"
	coll_symbolic "github.com/inoxlang/inox/internal/globals/containers/symbolic"
	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/stretchr/testify/assert"
)

func TestNewMap(t *testing.T) {

	t.Run("no elements", func(t *testing.T) {
		ctx := core.NewContexWithEmptyState(core.ContextConfig{}, io.Discard)
		m := NewMap(ctx, core.NewWrappedValueList(), nil)

		assert.Equal(t, MapConfig{
			Key:   core.SERIALIZABLE_PATTERN,
			Value: core.SERIALIZABLE_PATTERN,
		}, m.config)

	})

	t.Run("single entry", func(t *testing.T) {
		ctx := core.NewContexWithEmptyState(core.ContextConfig{}, io.Discard)
		m := NewMap(ctx, core.NewWrappedValueList(INT_1, STRING_A), nil)

		assert.Equal(t, MapConfig{
			Key:   core.SERIALIZABLE_PATTERN,
			Value: core.SERIALIZABLE_PATTERN,
		}, m.config)

		assert.True(t, bool(m.Has(ctx, INT_1)))
	})

	t.Run("element with no representation yet", func(t *testing.T) {
		ctx := core.NewContexWithEmptyState(core.ContextConfig{}, io.Discard)

		node := core.AstNode{Node: parse.MustParseChunk("")}

		func() {
			defer func() {
				assert.ErrorContains(t, recover().(error), "not implemented yet")
			}()
			NewMap(ctx, core.NewWrappedValueList(node, STRING_A), nil)
		}()
	})

	t.Run("element with representation should be immutable ", func(t *testing.T) {
		ctx := core.NewContexWithEmptyState(core.ContextConfig{}, io.Discard)

		obj := core.NewObjectFromMap(core.ValMap{}, ctx)

		func() {
			defer func() {
				assert.ErrorContains(t, recover().(error), core.ErrReprOfMutableValueCanChange.Error())
			}()
			NewMap(ctx, core.NewWrappedValueList(obj, STRING_A), nil)
		}()
	})

	t.Run("key pattern", func(t *testing.T) {
		ctx := core.NewContexWithEmptyState(core.ContextConfig{}, io.Discard)

		keyPattern := core.INT_PATTERN

		config := core.NewObjectFromMap(core.ValMap{
			coll_symbolic.MAP_CONFIG_KEY_PATTERN_KEY: keyPattern,
		}, ctx)

		m := NewMap(ctx, core.NewWrappedValueList(), core.ToOptionalParam(config))

		assert.Equal(t, MapConfig{
			Key:   keyPattern,
			Value: core.SERIALIZABLE_PATTERN,
		}, m.config)
	})

	t.Run("value pattern", func(t *testing.T) {
		ctx := core.NewContexWithEmptyState(core.ContextConfig{}, io.Discard)

		valuePattern := core.INT_PATTERN

		config := core.NewObjectFromMap(core.ValMap{
			coll_symbolic.MAP_CONFIG_VALUE_PATTERN_KEY: valuePattern,
		}, ctx)

		m := NewMap(ctx, core.NewWrappedValueList(), core.ToOptionalParam(config))

		assert.Equal(t, MapConfig{
			Key:   core.SERIALIZABLE_PATTERN,
			Value: valuePattern,
		}, m.config)
	})

	t.Run("value pattern: element does not match", func(t *testing.T) {
		ctx := core.NewContexWithEmptyState(core.ContextConfig{}, io.Discard)

		valuePattern := core.INT_PATTERN

		config := core.NewObjectFromMap(core.ValMap{
			coll_symbolic.MAP_CONFIG_VALUE_PATTERN_KEY: valuePattern,
		}, ctx)

		func() {
			defer func() {
				assert.ErrorContains(t, recover().(error), ErrValueDoesMatchValuePattern.Error())
			}()

			NewMap(ctx, core.NewWrappedValueList(core.Str("a"), core.True), core.ToOptionalParam(config))
		}()
	})
}

func TestUnsharedMapAddRemove(t *testing.T) {
	ctx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)

	t.Run("representation uniqueness", func(t *testing.T) {
		m := NewMapWithConfig(ctx, nil, MapConfig{
			Key:   core.INT_PATTERN,
			Value: core.SERIALIZABLE_PATTERN,
		})

		m.Insert(ctx, INT_1, STRING_A)
		assert.True(t, bool(m.Has(ctx, INT_1)))
		assert.False(t, bool(m.Has(ctx, INT_2)))

		m.Remove(ctx, INT_1)
		assert.False(t, bool(m.Has(ctx, INT_1)))
	})

	t.Run("representation uniquenesss: key with sensitive data", func(t *testing.T) {
		m := NewMapWithConfig(ctx, nil, MapConfig{
			Key:   core.RECORD_PATTERN,
			Value: core.SERIALIZABLE_PATTERN,
		})

		record := core.NewRecordFromMap(core.ValMap{"password": core.Str("x"), "email-address": core.EmailAddress("a@mail.com")})
		m.Insert(ctx, record, STRING_A)

		recordClone := utils.Must(core.RepresentationBasedClone(ctx, record))

		val, ok := m.Get(ctx, recordClone)

		if !assert.True(t, bool(ok)) {
			return
		}

		assert.Equal(t, STRING_A, val)
	})

}
