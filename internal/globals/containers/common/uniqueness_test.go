package containers_common

import (
	"testing"

	"github.com/inoxlang/inox/internal/core"
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
