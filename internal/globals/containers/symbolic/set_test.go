package containers

import (
	"testing"

	"github.com/inoxlang/inox/internal/core/symbolic"
	containers_common "github.com/inoxlang/inox/internal/globals/containers/common"
	"github.com/stretchr/testify/assert"
)

func TestSet(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		element := symbolic.NewTypePattern(symbolic.ANY_INT, nil, nil, nil)
		intSet1 := NewSetPatternWithElementPatternAndUniqueness(element, containers_common.NewReprUniqueness())
		intSet2 := NewSetPatternWithElementPatternAndUniqueness(element, containers_common.NewReprUniqueness())

		assert.True(t, intSet1.Test(intSet1, symbolic.RecTestCallState{}))
		assert.True(t, intSet1.Test(intSet2, symbolic.RecTestCallState{}))
	})
}
