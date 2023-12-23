package containers

import (
	"testing"

	"github.com/inoxlang/inox/internal/core/symbolic"
	containers_common "github.com/inoxlang/inox/internal/globals/containers/common"
	"github.com/stretchr/testify/assert"
)

func TestSet(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		intType := symbolic.NewTypePattern(symbolic.ANY_INT, nil, nil, nil)
		specificIntType := symbolic.NewTypePattern(symbolic.INT_1, nil, nil, nil)

		intSet1 := NewSetPatternWithElementPatternAndUniqueness(intType, containers_common.NewReprUniqueness())
		intSet2 := NewSetPatternWithElementPatternAndUniqueness(intType, containers_common.NewReprUniqueness())
		int1Set := NewSetPatternWithElementPatternAndUniqueness(specificIntType, containers_common.NewReprUniqueness())

		assert.True(t, intSet1.Test(intSet1, symbolic.RecTestCallState{}))
		assert.True(t, intSet1.Test(intSet2, symbolic.RecTestCallState{}))
		assert.True(t, intSet1.Test(int1Set, symbolic.RecTestCallState{}))

		assert.True(t, intSet1.SymbolicValue().Test(intSet1.SymbolicValue(), symbolic.RecTestCallState{}))
	})

	t.Run("TestValue()", func(t *testing.T) {
		intType := symbolic.NewTypePattern(symbolic.ANY_INT, nil, nil, nil)
		specificIntType := symbolic.NewTypePattern(symbolic.INT_1, nil, nil, nil)

		intSetPattern := NewSetPatternWithElementPatternAndUniqueness(intType, containers_common.NewReprUniqueness())
		intSet := NewSetWithPattern(intType, containers_common.NewReprUniqueness())
		int1Set := NewSetWithPattern(specificIntType, containers_common.NewReprUniqueness())

		assert.True(t, intSetPattern.TestValue(intSet, symbolic.RecTestCallState{}))
		assert.True(t, intSetPattern.TestValue(int1Set, symbolic.RecTestCallState{}))
		assert.True(t, intSetPattern.TestValue(intSetPattern.SymbolicValue(), symbolic.RecTestCallState{}))
	})
}
