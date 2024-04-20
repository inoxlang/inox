package setcoll

import (
	"testing"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/globals/containers/common"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/stretchr/testify/assert"

	containers_symbolic "github.com/inoxlang/inox/internal/globals/containers/symbolic"
)

func TestSetPattern(t *testing.T) {

	t.Run("", func(t *testing.T) {
		ctx := core.NewContextWithEmptyState(core.ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		patt, err := SET_PATTERN.Call(ctx, []core.Serializable{core.INT_PATTERN, common.REPR_UNIQUENESS_IDENT})

		if assert.NoError(t, err) {
			uniqueness := common.UniquenessConstraint{
				Type: common.UniqueRepr,
			}

			expectedPattern := NewSetPattern(SetConfig{
				Element:    core.INT_PATTERN,
				Uniqueness: uniqueness,
			})

			assert.Equal(t, expectedPattern, patt)
		}

		objectPattern := core.NewInexactObjectPattern([]core.ObjectPatternEntry{
			{
				Name:    "a",
				Pattern: core.INT_PATTERN,
			},
		})

		//
		patt, err = SET_PATTERN.Call(ctx, []core.Serializable{objectPattern, common.URL_UNIQUENESS_IDENT})

		if assert.NoError(t, err) {
			uniqueness := common.UniquenessConstraint{
				Type: common.UniqueURL,
			}

			expectedPattern := NewSetPattern(SetConfig{
				Element:    objectPattern,
				Uniqueness: uniqueness,
			})

			assert.Equal(t, expectedPattern, patt)
		}

		//
		patt, err = SET_PATTERN.Call(ctx, []core.Serializable{objectPattern, core.PropertyName("a")})

		if assert.NoError(t, err) {
			uniqueness := common.UniquenessConstraint{
				Type:         common.UniquePropertyValue,
				PropertyName: core.PropertyName("a"),
			}

			expectedPattern := NewSetPattern(SetConfig{
				Element:    objectPattern,
				Uniqueness: uniqueness,
			})

			assert.Equal(t, expectedPattern, patt)
		}
	})

	t.Run(".GetMigrationOperations", func(t *testing.T) {

		t.Run("uniqueness change", func(t *testing.T) {
			ctx := core.NewContextWithEmptyState(core.ContextConfig{}, nil)
			defer ctx.CancelGracefully()

			patt1, err1 := SET_PATTERN.Call(ctx, []core.Serializable{core.INT_PATTERN, common.REPR_UNIQUENESS_IDENT})
			patt2, err2 := SET_PATTERN.Call(ctx, []core.Serializable{core.INT_PATTERN, common.URL_UNIQUENESS_IDENT})

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
			ctx := core.NewContextWithEmptyState(core.ContextConfig{}, nil)
			defer ctx.CancelGracefully()

			patt1, err1 := SET_PATTERN.Call(ctx, []core.Serializable{core.INT_PATTERN, common.REPR_UNIQUENESS_IDENT})
			patt2, err2 := SET_PATTERN.Call(ctx, []core.Serializable{core.STR_PATTERN, common.REPR_UNIQUENESS_IDENT})

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
			ctx := core.NewContextWithEmptyState(core.ContextConfig{}, nil)
			defer ctx.CancelGracefully()

			patt1, err1 := SET_PATTERN.Call(ctx, []core.Serializable{core.INT_PATTERN, common.REPR_UNIQUENESS_IDENT})
			patt2, err2 := SET_PATTERN.Call(ctx, []core.Serializable{core.SERIALIZABLE_PATTERN, common.REPR_UNIQUENESS_IDENT})

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
			ctx := core.NewContextWithEmptyState(core.ContextConfig{}, nil)
			defer ctx.CancelGracefully()

			patt1, err1 := SET_PATTERN.Call(ctx, []core.Serializable{core.SERIALIZABLE_PATTERN, common.REPR_UNIQUENESS_IDENT})
			patt2, err2 := SET_PATTERN.Call(ctx, []core.Serializable{core.INT_PATTERN, common.REPR_UNIQUENESS_IDENT})

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

func TestSymbolicSetPattern(t *testing.T) {
	ctx := core.NewContextWithEmptyState(core.ContextConfig{}, nil)
	symbolicCtx := symbolic.NewSymbolicContext(ctx, nil, nil)

	t.Run("valid", func(t *testing.T) {
		intPattern := utils.Must(core.INT_PATTERN.ToSymbolicValue(ctx, map[uintptr]symbolic.Value{}))
		patt, err := SET_PATTERN.SymbolicCallImpl(symbolicCtx,
			[]symbolic.Value{intPattern, symbolic.NewIdentifier("repr")}, nil)

		if assert.NoError(t, err) {
			uniqueness := common.UniquenessConstraint{
				Type: common.UniqueRepr,
			}

			expectedPattern :=
				containers_symbolic.NewSetPatternWithElementPatternAndUniqueness(intPattern.(symbolic.Pattern), &uniqueness)

			assert.Equal(t, expectedPattern, patt)
		}
	})

	t.Run("invalid", func(t *testing.T) {
		mutableValuePattern := symbolic.NewInexactObjectPattern(map[string]symbolic.Pattern{}, nil)
		patt, err := SET_PATTERN.SymbolicCallImpl(symbolicCtx,
			[]symbolic.Value{mutableValuePattern, common.REPR_UNIQUENESS_SYMB_IDENT}, nil)

		if !assert.Error(t, err) {
			return
		}
		assert.Nil(t, patt)
	})

}
