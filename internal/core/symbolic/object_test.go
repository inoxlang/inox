package symbolic

import (
	"testing"

	utils "github.com/inoxlang/inox/internal/utils/common"
	"github.com/stretchr/testify/assert"
)

func TestObject(t *testing.T) {
	cases := []struct {
		object1 *Object
		object2 *Object
		ok      bool
	}{
		//an any object should match an any object
		{&Object{entries: nil}, &Object{entries: nil}, true},

		//an empty object should not match an any object
		{&Object{entries: map[string]Serializable{}}, &Object{entries: nil}, false},

		//an any object should match an empty object
		{&Object{entries: nil}, &Object{entries: map[string]Serializable{}}, true},

		//an empty object should match an empty object
		{&Object{entries: map[string]Serializable{}}, &Object{entries: map[string]Serializable{}}, true},

		//a readonly empty object should not match an empty object
		{&Object{entries: map[string]Serializable{}, readonly: true}, &Object{entries: map[string]Serializable{}}, false},

		//an empty object should not match a readonly empty object
		{&Object{entries: map[string]Serializable{}}, &Object{entries: map[string]Serializable{}, readonly: true}, false},

		{
			&Object{
				entries: map[string]Serializable{"a": ANY_INT},
			},
			&Object{
				entries: map[string]Serializable{},
			},
			false,
		},
		{
			&Object{
				entries: map[string]Serializable{},
			},
			&Object{
				entries: map[string]Serializable{"a": ANY_INT},
			},
			true,
		},
		{
			&Object{
				entries: map[string]Serializable{},
				exact:   true,
			},
			&Object{
				entries: map[string]Serializable{"a": ANY_INT},
			},
			false,
		},
		{
			&Object{
				entries:         map[string]Serializable{"a": ANY_INT},
				optionalEntries: map[string]struct{}{"a": {}},
			},
			&Object{
				entries: map[string]Serializable{},
			},
			true,
		},
		{
			&Object{
				entries: map[string]Serializable{"a": ANY_INT},
			},
			&Object{
				entries: map[string]Serializable{"a": ANY_INT},
			},
			true,
		},
		{
			&Object{
				entries: map[string]Serializable{"a": ANY_INT},
				exact:   true,
			},
			&Object{
				entries: map[string]Serializable{"a": ANY_INT},
			},
			false,
		},
		{
			&Object{
				entries: map[string]Serializable{"a": ANY_INT},
			},
			&Object{
				entries:         map[string]Serializable{"a": ANY_INT},
				optionalEntries: map[string]struct{}{"a": {}},
			},
			false,
		},
		{
			&Object{
				entries: map[string]Serializable{"a": ANY_INT},
				exact:   true,
			},
			&Object{
				entries:         map[string]Serializable{"a": ANY_INT},
				optionalEntries: map[string]struct{}{"a": {}},
			},
			false,
		},
		{
			&Object{
				entries: map[string]Serializable{"a": ANY_SERIALIZABLE},
			},
			&Object{
				entries: map[string]Serializable{"a": ANY_INT},
			},
			true,
		},
		{
			&Object{
				entries: map[string]Serializable{"a": ANY_INT},
			},
			&Object{
				entries: map[string]Serializable{"a": ANY_SERIALIZABLE},
			},
			false,
		},
	}

	for _, testCase := range cases {
		t.Run(t.Name()+"_"+Stringify(testCase.object1)+"_"+Stringify(testCase.object2), func(t *testing.T) {
			assert.Equal(t, testCase.ok, testCase.object1.Test(testCase.object2, RecTestCallState{}))
		})
	}

	t.Run("an infinite recursion should raise the error "+ErrMaximumSymbolicTestCallDepthReached.Error(), func(t *testing.T) {
		obj := &Object{}
		obj.entries = map[string]Serializable{
			"self": obj,
		}
		assert.PanicsWithError(t, ErrMaximumSymbolicTestCallDepthReached.Error(), func() {
			obj.Test(obj, RecTestCallState{})
		})
	})

	t.Run("Prop", func(t *testing.T) {
		t.Run("object has a URL witch a concrete value", func(t *testing.T) {
			inner := NewExactObject2(map[string]Serializable{
				"b": ANY_INT,
			})

			obj := NewExactObject2(map[string]Serializable{
				"a":     ANY_INT,
				"inner": inner,
			})

			obj = obj.WithURL(NewUrl("ldb://main/users/1")).(*Object)

			expectedInnerObjURL := NewUrl("ldb://main/users/1/inner")

			assert.Equal(t, ANY_INT, obj.Prop("a"))
			assert.Equal(t, inner.WithURL(expectedInnerObjURL), obj.Prop("inner"))
		})

		t.Run("object has a URL matching a pattern with a concrete value", func(t *testing.T) {
			inner := NewExactObject2(map[string]Serializable{
				"b": ANY_INT,
			})

			obj := NewExactObject2(map[string]Serializable{
				"a":     ANY_INT,
				"inner": inner,
			})

			url := NewUrlMatchingPattern(NewUrlPattern("ldb://main/users/%int"))
			obj = obj.WithURL(url).(*Object)

			expectedInnerObjURL := NewUrlMatchingPattern(NewUrlPattern("ldb://main/users/%int/inner"))

			assert.Equal(t, ANY_INT, obj.Prop("a"))
			assert.Equal(t, inner.WithURL(expectedInnerObjURL), obj.Prop("inner"))
		})
	})

	t.Run("SetProp", func(t *testing.T) {
		t.Run("should return an error if a new property is set in an  exact object", func(t *testing.T) {
			obj := NewExactObject(map[string]Serializable{}, nil, nil)
			updated, err := obj.SetProp(nil, nil, "new-prop", NewInt(1))
			if !assert.ErrorContains(t, err, CANNOT_ADD_NEW_PROPERTY_TO_AN_EXACT_OBJECT) {
				return
			}
			assert.Nil(t, updated)
		})
	})

	t.Run("ToReadonly()", func(t *testing.T) {

		t.Run("already readonly", func(t *testing.T) {
			object := NewInexactObject(map[string]Serializable{}, nil, nil)
			object.readonly = true

			result, err := object.ToReadonly()
			if !assert.NoError(t, err) {
				return
			}
			assert.Same(t, object, result)
		})

		t.Run("empty", func(t *testing.T) {
			object := NewInexactObject(map[string]Serializable{}, nil, nil)

			result, err := object.ToReadonly()
			if !assert.NoError(t, err) {
				return
			}

			expectedReadonly := NewInexactObject(map[string]Serializable{}, nil, nil)
			expectedReadonly.readonly = true

			assert.Equal(t, expectedReadonly, result)
		})

		t.Run("immutable property", func(t *testing.T) {
			object := NewInexactObject(map[string]Serializable{
				"x": ANY_TUPLE,
			}, nil, nil)

			result, err := object.ToReadonly()
			if !assert.NoError(t, err) {
				return
			}

			expectedReadonly := NewInexactObject(map[string]Serializable{
				"x": ANY_TUPLE,
			}, nil, nil)
			expectedReadonly.readonly = true

			assert.Equal(t, expectedReadonly, result)
		})

		t.Run("an error should be returned if a property is not convertible to readonly", func(t *testing.T) {
			object := NewInexactObject(map[string]Serializable{
				"x": ANY_SERIALIZABLE,
			}, nil, nil)

			result, err := object.ToReadonly()
			if !assert.ErrorIs(t, err, ErrNotConvertibleToReadonly) {
				return
			}

			assert.Nil(t, result)
		})
	})

	t.Run("SpecificIntersection", func(t *testing.T) {
		must := utils.Must[Value]
		t.Run("inexact", func(t *testing.T) {
			emptyInexact := NewInexactObject(map[string]Serializable{}, nil, nil)
			assert.Same(t, emptyInexact, must(emptyInexact.SpecificIntersection(emptyInexact, 0)))

			singlePropInexact := NewInexactObject(map[string]Serializable{"a": ANY_INT}, nil, nil)
			assert.Same(t, singlePropInexact, must(singlePropInexact.SpecificIntersection(singlePropInexact, 0)))

			assert.Equal(t, singlePropInexact, must(emptyInexact.SpecificIntersection(singlePropInexact, 0)))
			assert.Equal(t, singlePropInexact, must(singlePropInexact.SpecificIntersection(emptyInexact, 0)))
		})
	})

	t.Run("Contains()", func(t *testing.T) {
		assertMayContainButNotCertain(t, ANY_OBJ, ANY_SERIALIZABLE)
		assertMayContainButNotCertain(t, ANY_OBJ, ANY_INT)
		assertMayContainButNotCertain(t, ANY_OBJ, INT_1)

		assertMayContainButNotCertain(t, ANY_READONLY_OBJ, ANY_SERIALIZABLE)
		assertMayContainButNotCertain(t, ANY_READONLY_OBJ, ANY_INT)
		assertMayContainButNotCertain(t, ANY_READONLY_OBJ, INT_1)

		inexactObject := NewInexactObject2(map[string]Serializable{
			"a": ANY_INT,
		})

		assertMayContainButNotCertain(t, inexactObject, ANY_SERIALIZABLE)
		assertMayContainButNotCertain(t, inexactObject, ANY_INT)
		assertMayContainButNotCertain(t, inexactObject, INT_1)

		inexactObjectWithConcretizableValue := NewInexactObject2(map[string]Serializable{
			"a": INT_1,
		})

		assertContains(t, inexactObjectWithConcretizableValue, INT_1)
		assertMayContainButNotCertain(t, inexactObjectWithConcretizableValue, ANY_SERIALIZABLE)
		assertMayContainButNotCertain(t, inexactObjectWithConcretizableValue, ANY_INT)
		assertMayContainButNotCertain(t, inexactObjectWithConcretizableValue, INT_2)

		croncretizableExactObject := NewExactObject2(map[string]Serializable{
			"a": INT_1,
		})

		assertContains(t, croncretizableExactObject, INT_1)
		assertMayContainButNotCertain(t, croncretizableExactObject, ANY_SERIALIZABLE)
		assertMayContainButNotCertain(t, croncretizableExactObject, ANY_INT)
		assertCannotPossiblyContain(t, croncretizableExactObject, INT_2)
	})
}
