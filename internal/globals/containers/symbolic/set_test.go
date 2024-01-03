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

		anyElemAnyUniquenessSet := NewSetWithPattern(symbolic.ANY_PATTERN, nil)
		intAnyUniquenessSet := NewSetWithPattern(intType, nil)
		intSet1 := NewSetWithPattern(intType, common.NewReprUniqueness())
		intSet2 := NewSetWithPattern(intType, common.NewReprUniqueness())
		int1Set := NewSetWithPattern(specificIntType, common.NewReprUniqueness())

		assert.True(t, anyElemAnyUniquenessSet.Test(anyElemAnyUniquenessSet, symbolic.RecTestCallState{}))
		assert.True(t, anyElemAnyUniquenessSet.Test(intSet1, symbolic.RecTestCallState{}))
		assert.True(t, anyElemAnyUniquenessSet.Test(int1Set, symbolic.RecTestCallState{}))

		assert.True(t, intAnyUniquenessSet.Test(intAnyUniquenessSet, symbolic.RecTestCallState{}))
		assert.True(t, intAnyUniquenessSet.Test(intSet1, symbolic.RecTestCallState{}))
		assert.True(t, intAnyUniquenessSet.Test(int1Set, symbolic.RecTestCallState{}))

		assert.True(t, intSet1.Test(intSet1, symbolic.RecTestCallState{}))
		assert.True(t, intSet1.Test(intSet2, symbolic.RecTestCallState{}))
		assert.True(t, intSet1.Test(int1Set, symbolic.RecTestCallState{}))

		assert.False(t, intAnyUniquenessSet.Test(anyElemAnyUniquenessSet, symbolic.RecTestCallState{}))
		assert.False(t, intSet1.Test(anyElemAnyUniquenessSet, symbolic.RecTestCallState{}))
		assert.False(t, int1Set.Test(intSet1, symbolic.RecTestCallState{}))
		assert.False(t, int1Set.Test(intAnyUniquenessSet, symbolic.RecTestCallState{}))
	})
}

func TestSetPattern(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		intType := symbolic.NewTypePattern(symbolic.ANY_INT, nil, nil, nil)
		specificIntType := symbolic.NewTypePattern(symbolic.INT_1, nil, nil, nil)

		anyElemAnyUniquenessSetPatt := NewSetPatternWithElementPatternAndUniqueness(symbolic.ANY_PATTERN, nil)
		intAnyUniquenessSetPatt := NewSetPatternWithElementPatternAndUniqueness(intType, nil)
		intSetPatt1 := NewSetPatternWithElementPatternAndUniqueness(intType, common.NewReprUniqueness())
		intSetPatt2 := NewSetPatternWithElementPatternAndUniqueness(intType, common.NewReprUniqueness())
		int1SetPatt := NewSetPatternWithElementPatternAndUniqueness(specificIntType, common.NewReprUniqueness())

		assert.True(t, anyElemAnyUniquenessSetPatt.Test(anyElemAnyUniquenessSetPatt, symbolic.RecTestCallState{}))
		assert.True(t, anyElemAnyUniquenessSetPatt.Test(intSetPatt1, symbolic.RecTestCallState{}))
		assert.True(t, anyElemAnyUniquenessSetPatt.Test(int1SetPatt, symbolic.RecTestCallState{}))

		assert.True(t, intAnyUniquenessSetPatt.Test(intAnyUniquenessSetPatt, symbolic.RecTestCallState{}))
		assert.True(t, intAnyUniquenessSetPatt.Test(intSetPatt1, symbolic.RecTestCallState{}))
		assert.True(t, intAnyUniquenessSetPatt.Test(int1SetPatt, symbolic.RecTestCallState{}))

		assert.True(t, intSetPatt1.Test(intSetPatt1, symbolic.RecTestCallState{}))
		assert.True(t, intSetPatt1.Test(intSetPatt2, symbolic.RecTestCallState{}))
		assert.True(t, intSetPatt1.Test(int1SetPatt, symbolic.RecTestCallState{}))
		assert.True(t, intSetPatt1.SymbolicValue().Test(intSetPatt1.SymbolicValue(), symbolic.RecTestCallState{}))

		assert.False(t, int1SetPatt.Test(anyElemAnyUniquenessSetPatt, symbolic.RecTestCallState{}))
		assert.False(t, int1SetPatt.Test(intAnyUniquenessSetPatt, symbolic.RecTestCallState{}))
		assert.False(t, int1SetPatt.Test(intSetPatt1, symbolic.RecTestCallState{}))
		assert.False(t, intAnyUniquenessSetPatt.Test(anyElemAnyUniquenessSetPatt, symbolic.RecTestCallState{}))
	})

	t.Run("TestValue()", func(t *testing.T) {
		intType := symbolic.NewTypePattern(symbolic.ANY_INT, nil, nil, nil)
		specificIntType := symbolic.NewTypePattern(symbolic.INT_1, nil, nil, nil)

		anyElemAnyUniquenessSetPatt := NewSetPatternWithElementPatternAndUniqueness(symbolic.ANY_PATTERN, nil)
		intSetPattern := NewSetPatternWithElementPatternAndUniqueness(intType, common.NewReprUniqueness())
		intSet := NewSetWithPattern(intType, common.NewReprUniqueness())
		int1Set := NewSetWithPattern(specificIntType, common.NewReprUniqueness())

		assert.True(t, anyElemAnyUniquenessSetPatt.TestValue(anyElemAnyUniquenessSetPatt.SymbolicValue(), symbolic.RecTestCallState{}))
		assert.True(t, anyElemAnyUniquenessSetPatt.TestValue(intSet, symbolic.RecTestCallState{}))
		assert.True(t, anyElemAnyUniquenessSetPatt.TestValue(int1Set, symbolic.RecTestCallState{}))

		assert.True(t, intSetPattern.TestValue(intSet, symbolic.RecTestCallState{}))
		assert.True(t, intSetPattern.TestValue(int1Set, symbolic.RecTestCallState{}))
		assert.True(t, intSetPattern.TestValue(intSetPattern.SymbolicValue(), symbolic.RecTestCallState{}))

		assert.False(t, intSetPattern.TestValue(anyElemAnyUniquenessSetPatt.SymbolicValue(), symbolic.RecTestCallState{}))
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
