package containers

import (
	"testing"

	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/globals/containers/common"
	"github.com/stretchr/testify/assert"
)

func TestSet(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		intType := symbolic.NewTypePattern(symbolic.ANY_INT, nil, nil, nil)
		specificIntType := symbolic.NewTypePattern(symbolic.INT_1, nil, nil, nil)

		intSet1 := NewSetPatternWithElementPatternAndUniqueness(intType, common.NewReprUniqueness())
		intSet2 := NewSetPatternWithElementPatternAndUniqueness(intType, common.NewReprUniqueness())
		int1Set := NewSetPatternWithElementPatternAndUniqueness(specificIntType, common.NewReprUniqueness())

		assert.True(t, intSet1.Test(intSet1, symbolic.RecTestCallState{}))
		assert.True(t, intSet1.Test(intSet2, symbolic.RecTestCallState{}))
		assert.True(t, intSet1.Test(int1Set, symbolic.RecTestCallState{}))

		assert.True(t, intSet1.SymbolicValue().Test(intSet1.SymbolicValue(), symbolic.RecTestCallState{}))
	})

	t.Run("TestValue()", func(t *testing.T) {
		intType := symbolic.NewTypePattern(symbolic.ANY_INT, nil, nil, nil)
		specificIntType := symbolic.NewTypePattern(symbolic.INT_1, nil, nil, nil)

		intSetPattern := NewSetPatternWithElementPatternAndUniqueness(intType, common.NewReprUniqueness())
		intSet := NewSetWithPattern(intType, common.NewReprUniqueness())
		int1Set := NewSetWithPattern(specificIntType, common.NewReprUniqueness())

		assert.True(t, intSetPattern.TestValue(intSet, symbolic.RecTestCallState{}))
		assert.True(t, intSetPattern.TestValue(int1Set, symbolic.RecTestCallState{}))
		assert.True(t, intSetPattern.TestValue(intSetPattern.SymbolicValue(), symbolic.RecTestCallState{}))
	})

	t.Run("elements' URL", func(t *testing.T) {
		t.Run("base case", func(t *testing.T) {
			objPattern := symbolic.NewInexactObjectPattern(map[string]symbolic.Pattern{}, nil)
			obj := objPattern.SymbolicValue().(*symbolic.Object)
			set := NewSetWithPattern(objPattern, common.NewURLUniqueness())
			set = set.WithURL(symbolic.NewUrl("ldb://main/users")).(*Set)

			elem, _ := set.Get(nil, symbolic.ANY_STR)
			if !assert.NotNil(t, elem) {
				return
			}

			urlPattern := symbolic.NewUrlPattern("ldb://main/users/*")
			expectedElement := obj.WithURL(symbolic.NewUrlMatchingPattern(urlPattern))
			assert.Equal(t, expectedElement, elem)
		})

		t.Run("multivalue element", func(t *testing.T) {
			obj1 := symbolic.NewExactObject2(map[string]symbolic.Serializable{"a": symbolic.ANY_INT})
			obj2 := symbolic.NewExactObject2(map[string]symbolic.Serializable{"b": symbolic.ANY_INT})

			multiValue := symbolic.NewMultivalue(obj1, obj2)
			elementPattern := symbolic.NewTypePattern(multiValue, nil, nil, nil)

			set := NewSetWithPattern(elementPattern, common.NewURLUniqueness())
			set = set.WithURL(symbolic.NewUrl("ldb://main/users")).(*Set)

			elem, _ := set.Get(nil, symbolic.ANY_STR)
			if !assert.NotNil(t, elem) {
				return
			}

			urlPattern := symbolic.NewUrlPattern("ldb://main/users/*")
			expectedObj1 := obj1.WithURL(symbolic.NewUrlMatchingPattern(urlPattern))
			expectedObj2 := obj2.WithURL(symbolic.NewUrlMatchingPattern(urlPattern))

			expectedElement := symbolic.NewMultivalue(expectedObj1, expectedObj2)
			assert.Equal(t, expectedElement, elem)
		})
	})
}
