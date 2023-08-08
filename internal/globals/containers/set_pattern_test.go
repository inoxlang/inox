package containers

import (
	"testing"

	"github.com/inoxlang/inox/internal/core"
	containers_common "github.com/inoxlang/inox/internal/globals/containers/common"
	"github.com/stretchr/testify/assert"
)

func TestSetPattern(t *testing.T) {

	t.Run("", func(t *testing.T) {
		//
		patt, err := SET_PATTERN.Call([]core.Serializable{core.INT_PATTERN, containers_common.REPR_UNIQUENESS_IDENT})

		if assert.NoError(t, err) {
			uniqueness := containers_common.UniquenessConstraint{
				Type: containers_common.UniqueRepr,
			}

			expectedPattern := NewSetPattern(SetConfig{
				Element:    core.INT_PATTERN,
				Uniqueness: uniqueness,
			}, core.CallBasedPatternReprMixin{
				Callee: SET_PATTERN,
				Params: []core.Serializable{core.INT_PATTERN, uniqueness.ToValue()},
			})

			assert.Equal(t, expectedPattern, patt)
		}

		objectPattern := core.NewInexactObjectPattern(map[string]core.Pattern{
			"a": core.INT_PATTERN,
		})

		//
		patt, err = SET_PATTERN.Call([]core.Serializable{objectPattern, containers_common.URL_UNIQUENESS_IDENT})

		if assert.NoError(t, err) {
			uniqueness := containers_common.UniquenessConstraint{
				Type: containers_common.UniqueURL,
			}

			expectedPattern := NewSetPattern(SetConfig{
				Element:    objectPattern,
				Uniqueness: uniqueness,
			}, core.CallBasedPatternReprMixin{
				Callee: SET_PATTERN,
				Params: []core.Serializable{objectPattern, uniqueness.ToValue()},
			})

			assert.Equal(t, expectedPattern, patt)
		}

		//
		patt, err = SET_PATTERN.Call([]core.Serializable{objectPattern, core.PropertyName("a")})

		if assert.NoError(t, err) {
			uniqueness := containers_common.UniquenessConstraint{
				Type:         containers_common.UniquePropertyValue,
				PropertyName: core.PropertyName("a"),
			}

			expectedPattern := NewSetPattern(SetConfig{
				Element:    objectPattern,
				Uniqueness: uniqueness,
			}, core.CallBasedPatternReprMixin{
				Callee: SET_PATTERN,
				Params: []core.Serializable{objectPattern, uniqueness.ToValue()},
			})

			assert.Equal(t, expectedPattern, patt)
		}
	})

	t.Run(".GetMigrationOperations", func(t *testing.T) {

		t.Run("uniqueness change", func(t *testing.T) {
			ctx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
			patt1, err1 := SET_PATTERN.Call([]core.Serializable{core.INT_PATTERN, containers_common.REPR_UNIQUENESS_IDENT})
			patt2, err2 := SET_PATTERN.Call([]core.Serializable{core.INT_PATTERN, containers_common.URL_UNIQUENESS_IDENT})

			if !assert.NoError(t, err1) {
				return
			}

			if !assert.NoError(t, err2) {
				return
			}

			migrations, err := patt1.(*SetPattern).GetMigrationOperations(ctx, patt2, "/")

			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, []core.MigrationOp{
				core.ReplacementMigrationOp{
					Current:        patt1,
					Next:           patt2,
					MigrationMixin: core.MigrationMixin{PseudoPath: "/"},
				},
			}, migrations)
		})

		t.Run("element pattern replaced with different type", func(t *testing.T) {
			ctx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
			patt1, err1 := SET_PATTERN.Call([]core.Serializable{core.INT_PATTERN, containers_common.REPR_UNIQUENESS_IDENT})
			patt2, err2 := SET_PATTERN.Call([]core.Serializable{core.STR_PATTERN, containers_common.REPR_UNIQUENESS_IDENT})

			if !assert.NoError(t, err1) {
				return
			}

			if !assert.NoError(t, err2) {
				return
			}

			migrations, err := patt1.(*SetPattern).GetMigrationOperations(ctx, patt2, "/")

			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, []core.MigrationOp{
				core.ReplacementMigrationOp{
					Current:        core.INT_PATTERN,
					Next:           core.STR_PATTERN,
					MigrationMixin: core.MigrationMixin{PseudoPath: "/*"},
				},
			}, migrations)
		})

		t.Run("element pattern replaced with super type", func(t *testing.T) {
			ctx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
			patt1, err1 := SET_PATTERN.Call([]core.Serializable{core.INT_PATTERN, containers_common.REPR_UNIQUENESS_IDENT})
			patt2, err2 := SET_PATTERN.Call([]core.Serializable{core.SERIALIZABLE_PATTERN, containers_common.REPR_UNIQUENESS_IDENT})

			if !assert.NoError(t, err1) {
				return
			}

			if !assert.NoError(t, err2) {
				return
			}

			migrations, err := patt1.(*SetPattern).GetMigrationOperations(ctx, patt2, "/")

			if !assert.NoError(t, err) {
				return
			}

			assert.Empty(t, migrations)
		})

		t.Run("element pattern replaced with sub type", func(t *testing.T) {
			ctx := core.NewContexWithEmptyState(core.ContextConfig{}, nil)
			patt1, err1 := SET_PATTERN.Call([]core.Serializable{core.SERIALIZABLE_PATTERN, containers_common.REPR_UNIQUENESS_IDENT})
			patt2, err2 := SET_PATTERN.Call([]core.Serializable{core.INT_PATTERN, containers_common.REPR_UNIQUENESS_IDENT})

			if !assert.NoError(t, err1) {
				return
			}

			if !assert.NoError(t, err2) {
				return
			}

			migrations, err := patt1.(*SetPattern).GetMigrationOperations(ctx, patt2, "/")

			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, []core.MigrationOp{
				core.ReplacementMigrationOp{
					Current:        core.SERIALIZABLE_PATTERN,
					Next:           core.INT_PATTERN,
					MigrationMixin: core.MigrationMixin{PseudoPath: "/*"},
				},
			}, migrations)
		})

	})
}
