package symbolic

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPropertyName(t *testing.T) {

	t.Run("any", func(t *testing.T) {
		anyPropName := ANY_PROPNAME
		iprops := NewInexactObject2(map[string]Serializable{"a": ANY_INT})
		assert.False(t, anyPropName.IsConcretizable())

		t.Run("Test", func(t *testing.T) {
			assertTest(t, anyPropName, anyPropName)
			assertTest(t, anyPropName, NewPropertyName("a"))
		})

		t.Run("GetFrom", func(t *testing.T) {
			result, alwaysPresent, err := anyPropName.GetFrom(iprops)
			assert.Nil(t, result)
			assert.False(t, alwaysPresent)
			assert.ErrorIs(t, err, ErrNotConcretizable)

			result, alwaysPresent, err = anyPropName.GetFrom(Nil)
			assert.Nil(t, result)
			assert.False(t, alwaysPresent)
			assert.ErrorIs(t, err, ErrNotConcretizable)
		})
	})

	t.Run("concretizable", func(t *testing.T) {
		propNameA := NewPropertyName("a")
		propNameB := NewPropertyName("b")

		iprops := NewInexactObject2(map[string]Serializable{"a": ANY_INT})

		assert.True(t, propNameA.IsConcretizable())

		t.Run("Test", func(t *testing.T) {
			assertTest(t, propNameA, propNameA)
			assertTestFalse(t, propNameA, propNameB)
			assertTestFalse(t, propNameA, ANY_PROPNAME)
		})

		t.Run("GetFrom", func(t *testing.T) {
			//propNameA
			result, alwaysPresent, err := propNameA.GetFrom(iprops)
			if !assert.Equal(t, ANY_INT, result) {
				return
			}
			assert.True(t, alwaysPresent)
			assert.Nil(t, err)

			result, alwaysPresent, err = propNameA.GetFrom(Nil)
			assert.Nil(t, result)
			assert.False(t, alwaysPresent)
			assert.Nil(t, err)

			//propNameB
			result, alwaysPresent, err = propNameB.GetFrom(iprops)
			assert.Nil(t, result)
			assert.False(t, alwaysPresent)
			assert.Nil(t, err)
		})
	})
}

func TestLongValuePath(t *testing.T) {

	t.Run("any", func(t *testing.T) {
		anyLongValuePath := ANY_LONG_VALUE_PATH
		iprops := NewInexactObject2(map[string]Serializable{
			"a": NewInexactObject2(map[string]Serializable{"b": ANY_INT}),
		})

		assert.False(t, anyLongValuePath.IsConcretizable())

		t.Run("Test", func(t *testing.T) {
			assertTest(t, anyLongValuePath, anyLongValuePath)
			assertTest(t, anyLongValuePath, NewLongValuePath(NewPropertyName("a"), NewPropertyName("b")))
			assertTest(t, anyLongValuePath, NewLongValuePath(ANY_PROPNAME, NewPropertyName("b")))
		})

		t.Run("GetFrom", func(t *testing.T) {
			result, alwaysPresent, err := anyLongValuePath.GetFrom(iprops)
			assert.Nil(t, result)
			assert.False(t, alwaysPresent)
			assert.ErrorIs(t, err, ErrNotConcretizable)

			result, alwaysPresent, err = anyLongValuePath.GetFrom(Nil)
			assert.Nil(t, result)
			assert.False(t, alwaysPresent)
			assert.ErrorIs(t, err, ErrNotConcretizable)
		})
	})

	t.Run("concretizable", func(t *testing.T) {
		longValuePathAB := NewLongValuePath(NewPropertyName("a"), NewPropertyName("b"))
		longValuePathBA := NewLongValuePath(NewPropertyName("b"), NewPropertyName("a"))

		iprops := NewInexactObject2(map[string]Serializable{
			"a": NewInexactObject2(map[string]Serializable{"b": ANY_INT}),
		})

		assert.True(t, longValuePathAB.IsConcretizable())

		t.Run("Test", func(t *testing.T) {
			assertTest(t, longValuePathAB, longValuePathAB)
			assertTestFalse(t, longValuePathAB, longValuePathBA)
			assertTestFalse(t, longValuePathAB, ANY_PROPNAME)
		})

		t.Run("GetFrom", func(t *testing.T) {
			//longValuepathAB
			result, alwaysPresent, err := longValuePathAB.GetFrom(iprops)
			if !assert.Equal(t, ANY_INT, result) {
				return
			}
			assert.True(t, alwaysPresent)
			assert.Nil(t, err)

			result, alwaysPresent, err = longValuePathAB.GetFrom(Nil)
			assert.Nil(t, result)
			assert.False(t, alwaysPresent)
			assert.Nil(t, err)

			//longValuepathBA
			result, alwaysPresent, err = longValuePathBA.GetFrom(iprops)
			assert.Nil(t, result)
			assert.False(t, alwaysPresent)
			assert.Nil(t, err)
		})
	})
}
