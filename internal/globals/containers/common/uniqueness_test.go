package containers_common

import (
	"testing"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/stretchr/testify/assert"
)

func TestUniquenessConstraint(t *testing.T) {

	t.Run("", func(t *testing.T) {
		assert.True(t,
			(UniquenessConstraint{
				Type:         UniquePropertyValue,
				PropertyName: core.PropertyName("a"),
			}).Equal(UniquenessConstraint{
				Type:         UniquePropertyValue,
				PropertyName: core.PropertyName("a"),
			}),
		)

		assert.True(t,
			(UniquenessConstraint{
				Type: UniqueURL,
			}).Equal(UniquenessConstraint{
				Type: UniqueURL,
			}),
		)

		assert.True(t,
			(UniquenessConstraint{
				Type: UniqueRepr,
			}).Equal(UniquenessConstraint{
				Type: UniqueRepr,
			}),
		)

		assert.False(t,
			(UniquenessConstraint{
				Type: UniqueURL,
			}).Equal(UniquenessConstraint{
				Type:         UniquePropertyValue,
				PropertyName: core.PropertyName("a"),
			}),
		)

		assert.False(t,
			(UniquenessConstraint{
				Type:         UniquePropertyValue,
				PropertyName: core.PropertyName("a"),
			}).Equal(UniquenessConstraint{
				Type: UniqueURL,
			}),
		)

		assert.False(t,
			(UniquenessConstraint{
				Type: UniqueURL,
			}).Equal(UniquenessConstraint{
				Type: UniqueRepr,
			}),
		)

		assert.False(t,
			(UniquenessConstraint{
				Type: UniqueRepr,
			}).Equal(UniquenessConstraint{
				Type: UniqueURL,
			}),
		)

		assert.False(t,
			(UniquenessConstraint{
				Type:         UniquePropertyValue,
				PropertyName: core.PropertyName("a"),
			}).Equal(UniquenessConstraint{
				Type:         UniquePropertyValue,
				PropertyName: core.PropertyName("b"),
			}),
		)

		assert.False(t,
			(UniquenessConstraint{
				Type:         UniquePropertyValue,
				PropertyName: core.PropertyName("b"),
			}).Equal(UniquenessConstraint{
				Type:         UniquePropertyValue,
				PropertyName: core.PropertyName("a"),
			}),
		)
	})
}

func TestUniquenessConstraintFromSymbolicValue(t *testing.T) {
	t.Run("property-based uniqueness requires values to have the property", func(t *testing.T) {
		objectPattern := symbolic.NewInexactObjectPattern(map[string]symbolic.Pattern{}, nil)

		propertyName := symbolic.NewPropertyName(".x")
		uniqueness, err := UniquenessConstraintFromSymbolicValue(propertyName, objectPattern)
		if !assert.ErrorIs(t, err, ErrPropertyBasedUniquenessRequireValuesToHaveTheProperty) {
			return
		}

		assert.Zero(t, uniqueness)
	})

	t.Run("representation-based uniqueness requires values to be immutable", func(t *testing.T) {
		mutableValuePattern := symbolic.NewInexactObjectPattern(map[string]symbolic.Pattern{}, nil)

		uniqueness, err := UniquenessConstraintFromSymbolicValue(REPR_UNIQUENESS_SYMB_IDENT, mutableValuePattern)
		if !assert.ErrorIs(t, err, ErrReprBasedUniquenessRequireValuesToBeImmutable) {
			return
		}

		assert.Zero(t, uniqueness)
	})

	t.Run("URL-based uniqueness requires values to be URL holders", func(t *testing.T) {
		nonUrlHolderPattern := symbolic.ANY_RECORD_PATTERN

		uniqueness, err := UniquenessConstraintFromSymbolicValue(URL_UNIQUENESS_SYMB_IDENT, nonUrlHolderPattern)
		if !assert.ErrorIs(t, err, ErrUrlBasedUniquenessRequireValuesToBeUrlHolders) {
			return
		}

		assert.Zero(t, uniqueness)
	})
}
