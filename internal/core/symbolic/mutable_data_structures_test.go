package symbolic

import (
	"context"
	"fmt"
	"testing"

	"github.com/inoxlang/inox/internal/utils"
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
			updated, err := obj.SetProp("new-prop", NewInt(1))
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

func TestList(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		cases := []struct {
			list1 *List
			list2 *List
			ok    bool
		}{
			{
				&List{elements: nil, generalElement: ANY_SERIALIZABLE},
				&List{elements: nil, generalElement: ANY_SERIALIZABLE},
				true,
			},
			{
				&List{elements: nil, generalElement: ANY_SERIALIZABLE},
				&List{elements: nil, generalElement: ANY_SERIALIZABLE, readonly: true},
				false,
			},
			{
				&List{elements: nil, generalElement: ANY_SERIALIZABLE, readonly: true},
				&List{elements: nil, generalElement: ANY_SERIALIZABLE},
				false,
			},
			{
				&List{elements: nil, generalElement: ANY_SERIALIZABLE},
				&List{elements: nil, generalElement: ANY_INT},
				true,
			},
			{
				&List{elements: nil, generalElement: ANY_INT},
				&List{elements: nil, generalElement: ANY_SERIALIZABLE},
				false,
			},
			{
				&List{elements: nil, generalElement: ANY_INT},
				&List{elements: nil, generalElement: ANY_INT},
				true,
			},
			{
				&List{elements: nil, generalElement: ANY_INT},
				&List{elements: []Serializable{ANY_INT}, generalElement: nil},
				true,
			},
			{
				&List{elements: nil, generalElement: ANY_INT, readonly: true},
				&List{elements: []Serializable{ANY_INT}, generalElement: nil},
				false,
			},
			{
				&List{elements: nil, generalElement: ANY_INT},
				&List{elements: []Serializable{ANY_INT}, generalElement: nil, readonly: true},
				false,
			},
			{
				&List{elements: nil, generalElement: ANY_INT},
				&List{elements: []Serializable{ANY_INT, ANY_BOOL}, generalElement: nil},
				false,
			},
			{
				&List{elements: []Serializable{ANY_INT}, generalElement: nil},
				&List{elements: nil, generalElement: ANY_INT},
				false,
			},
			{
				&List{elements: []Serializable{ANY_INT}, generalElement: nil},
				&List{elements: []Serializable{ANY_INT}, generalElement: nil},
				true,
			},
			{
				&List{elements: []Serializable{ANY_INT}, generalElement: nil},
				&List{elements: []Serializable{ANY_BOOL}, generalElement: nil},
				false,
			},
			{
				&List{elements: []Serializable{ANY_INT, ANY_INT}, generalElement: nil},
				&List{elements: []Serializable{ANY_INT, ANY_BOOL}, generalElement: nil},
				false,
			},
		}

		for _, testCase := range cases {
			t.Run(fmt.Sprint(testCase.list1, "_", testCase.list2), func(t *testing.T) {
				assert.Equal(t, testCase.ok, testCase.list1.Test(testCase.list2, RecTestCallState{}))
			})
		}

		t.Run("an infinite recursion should raise the error "+ErrMaximumSymbolicTestCallDepthReached.Error(), func(t *testing.T) {
			list1 := &List{}
			list1.elements = []Serializable{list1}
			assert.PanicsWithError(t, ErrMaximumSymbolicTestCallDepthReached.Error(), func() {
				list1.Test(list1, RecTestCallState{})
			})

			list2 := &List{}
			list2.generalElement = list2
			assert.PanicsWithError(t, ErrMaximumSymbolicTestCallDepthReached.Error(), func() {
				list2.Test(list2, RecTestCallState{})
			})
		})
	})

	t.Run("Append()", func(t *testing.T) {
		t.Run("adding no elements to an empty list", func(t *testing.T) {
			ctx := NewSymbolicContext(dummyConcreteContext{context.Background()}, nil, nil)
			state := newSymbolicState(ctx, nil)

			list := NewList()
			list.Append(ctx)

			_, ok := state.consumeUpdatedSelf()
			assert.False(t, ok)
		})

		t.Run("adding a single element to an empty list", func(t *testing.T) {
			ctx := NewSymbolicContext(dummyConcreteContext{context.Background()}, nil, nil)
			state := newSymbolicState(ctx, nil)

			list := NewList()
			list.Append(ctx, NewInt(1))

			updatedSelf, ok := state.consumeUpdatedSelf()
			if !assert.True(t, ok) {
				return
			}

			assert.Equal(t, NewListOf(INT_1), updatedSelf)
		})

		t.Run("adding two different elements of the same type to an empty list", func(t *testing.T) {
			ctx := NewSymbolicContext(dummyConcreteContext{context.Background()}, nil, nil)
			state := newSymbolicState(ctx, nil)

			list := NewList()
			list.Append(ctx, NewInt(1), NewInt(2))

			updatedSelf, ok := state.consumeUpdatedSelf()
			if !assert.True(t, ok) {
				return
			}

			assert.Equal(t, NewListOf(AsSerializableChecked(NewMultivalue(INT_1, INT_2))), updatedSelf)
		})

		t.Run("adding no element to a list with single element", func(t *testing.T) {
			ctx := NewSymbolicContext(dummyConcreteContext{context.Background()}, nil, nil)
			state := newSymbolicState(ctx, nil)

			list := NewList(NewInt(1))
			list.Append(ctx)

			_, ok := state.consumeUpdatedSelf()
			assert.False(t, ok)
		})

		t.Run("adding same static type element to list with single element", func(t *testing.T) {
			ctx := NewSymbolicContext(dummyConcreteContext{context.Background()}, nil, nil)
			state := newSymbolicState(ctx, nil)

			list := NewList(NewInt(1))
			list.Append(ctx, NewInt(2))

			updatedSelf, ok := state.consumeUpdatedSelf()
			if !assert.True(t, ok) {
				return
			}

			assert.Equal(t, NewListOf(ANY_INT), updatedSelf)
		})

		t.Run("adding same static type element to list with two elements", func(t *testing.T) {
			ctx := NewSymbolicContext(dummyConcreteContext{context.Background()}, nil, nil)
			state := newSymbolicState(ctx, nil)

			list := NewList(NewInt(1), NewInt(2))
			list.Append(ctx, NewInt(3))

			updatedSelf, ok := state.consumeUpdatedSelf()
			if !assert.True(t, ok) {
				return
			}

			assert.Equal(t, NewListOf(ANY_INT), updatedSelf)
		})
	})

	t.Run("Pop()", func(t *testing.T) {
		t.Run("empty list", func(t *testing.T) {
			ctx := NewSymbolicContext(dummyConcreteContext{context.Background()}, nil, nil)
			state := newSymbolicState(ctx, nil)

			list := NewList()
			list.Pop(ctx)

			err := false
			state.consumeSymbolicGoFunctionErrors(func(msg string) {
				err = true
				assert.Equal(t, CANNOT_POP_FROM_EMPTY_LIST, msg)
			})

			assert.True(t, err)
			_, ok := state.consumeUpdatedSelf()
			assert.False(t, ok)
		})

		t.Run("list of known length 1", func(t *testing.T) {
			ctx := NewSymbolicContext(dummyConcreteContext{context.Background()}, nil, nil)
			state := newSymbolicState(ctx, nil)

			list := NewList(INT_1)
			list.Pop(ctx)

			updatedSelf, ok := state.consumeUpdatedSelf()
			if !assert.True(t, ok) {
				return
			}

			assert.Equal(t, NewList(), updatedSelf)

			state.consumeSymbolicGoFunctionErrors(func(msg string) {
				assert.Fail(t, "unexcepted error: "+msg)
			})
		})

		t.Run("list of known length 2", func(t *testing.T) {
			ctx := NewSymbolicContext(dummyConcreteContext{context.Background()}, nil, nil)
			state := newSymbolicState(ctx, nil)

			list := NewList(INT_1, INT_2)
			list.Pop(ctx)

			updatedSelf, ok := state.consumeUpdatedSelf()
			if !assert.True(t, ok) {
				return
			}

			assert.Equal(t, NewList(INT_1), updatedSelf)

			state.consumeSymbolicGoFunctionErrors(func(msg string) {
				assert.Fail(t, "unexcepted error: "+msg)
			})
		})

		t.Run("list of unknown length", func(t *testing.T) {
			ctx := NewSymbolicContext(dummyConcreteContext{context.Background()}, nil, nil)
			state := newSymbolicState(ctx, nil)

			list := NewListOf(ANY_INT)
			list.Pop(ctx)

			_, ok := state.consumeUpdatedSelf()
			assert.False(t, ok)

			state.consumeSymbolicGoFunctionErrors(func(msg string) {
				assert.Fail(t, "unexcepted error: "+msg)
			})
		})

	})

	t.Run("ToReadonly()", func(t *testing.T) {

		t.Run("already readonly", func(t *testing.T) {
			list := NewList()
			list.readonly = true

			result, err := list.ToReadonly()
			if !assert.NoError(t, err) {
				return
			}
			assert.Same(t, list, result)
		})

		t.Run("empty", func(t *testing.T) {
			list := NewList()

			result, err := list.ToReadonly()
			if !assert.NoError(t, err) {
				return
			}

			expectedReadonly := NewList()
			expectedReadonly.readonly = true

			assert.Equal(t, expectedReadonly, result)
		})

		t.Run("immutable element", func(t *testing.T) {
			list := NewList(ANY_TUPLE)

			result, err := list.ToReadonly()
			if !assert.NoError(t, err) {
				return
			}

			expectedReadonly := NewList(ANY_TUPLE)
			expectedReadonly.readonly = true

			assert.Equal(t, expectedReadonly, result)
		})

		t.Run("an error should be returned if an element is not convertible to readonly", func(t *testing.T) {
			object := NewList(ANY_SERIALIZABLE)

			result, err := object.ToReadonly()
			if !assert.ErrorIs(t, err, ErrNotConvertibleToReadonly) {
				return
			}

			assert.Nil(t, result)
		})
	})

	t.Run("Contains()", func(t *testing.T) {
		intList := NewListOf(ANY_INT)
		assertMayContainButNotCertain(t, intList, ANY_SERIALIZABLE)
		assertMayContainButNotCertain(t, intList, ANY_INT)
		assertMayContainButNotCertain(t, intList, INT_1)
		assertMayContainButNotCertain(t, intList, INT_2)

		listWithConcreteValue := NewListOf(INT_1)
		assertContains(t, listWithConcreteValue, INT_1)
		assertMayContainButNotCertain(t, listWithConcreteValue, ANY_SERIALIZABLE)
		assertMayContainButNotCertain(t, listWithConcreteValue, ANY_INT)
		assertCannotPossiblyContain(t, listWithConcreteValue, INT_2)

		anyIntPair := NewList(ANY_INT, ANY_INT)
		assertMayContainButNotCertain(t, anyIntPair, ANY_SERIALIZABLE)
		assertMayContainButNotCertain(t, anyIntPair, ANY_INT)
		assertMayContainButNotCertain(t, anyIntPair, INT_1)
		assertMayContainButNotCertain(t, anyIntPair, INT_2)

		concretizableList := NewList(INT_1, INT_2)
		assertContains(t, concretizableList, INT_1)
		assertContains(t, concretizableList, INT_2)
		assertMayContainButNotCertain(t, concretizableList, ANY_SERIALIZABLE)
		assertMayContainButNotCertain(t, concretizableList, ANY_INT)
		assertCannotPossiblyContain(t, concretizableList, INT_3)
	})
}

func TestArray(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {
		cases := []struct {
			array1 *Array
			array2 *Array
			ok     bool
		}{
			{
				&Array{elements: nil, generalElement: ANY_SERIALIZABLE},
				&Array{elements: nil, generalElement: ANY_SERIALIZABLE},
				true,
			},
			{
				&Array{elements: nil, generalElement: ANY_SERIALIZABLE},
				&Array{elements: nil, generalElement: ANY_INT},
				true,
			},
			{
				&Array{elements: nil, generalElement: ANY_INT},
				&Array{elements: nil, generalElement: ANY_SERIALIZABLE},
				false,
			},
			{
				&Array{elements: nil, generalElement: ANY_INT},
				&Array{elements: nil, generalElement: ANY_INT},
				true,
			},
			{
				&Array{elements: nil, generalElement: ANY_INT},
				&Array{elements: []Value{ANY_INT}, generalElement: nil},
				true,
			},
			{
				&Array{elements: nil, generalElement: ANY_INT},
				&Array{elements: []Value{ANY_INT, ANY_BOOL}, generalElement: nil},
				false,
			},
			{
				&Array{elements: []Value{ANY_INT}, generalElement: nil},
				&Array{elements: nil, generalElement: ANY_INT},
				false,
			},
			{
				&Array{elements: []Value{ANY_INT}, generalElement: nil},
				&Array{elements: []Value{ANY_INT}, generalElement: nil},
				true,
			},
			{
				&Array{elements: []Value{ANY_INT}, generalElement: nil},
				&Array{elements: []Value{ANY_BOOL}, generalElement: nil},
				false,
			},
			{
				&Array{elements: []Value{ANY_INT, ANY_INT}, generalElement: nil},
				&Array{elements: []Value{ANY_INT, ANY_BOOL}, generalElement: nil},
				false,
			},
		}

		for _, testCase := range cases {
			t.Run(fmt.Sprint(testCase.array1, "_", testCase.array2), func(t *testing.T) {
				assert.Equal(t, testCase.ok, testCase.array1.Test(testCase.array2, RecTestCallState{}))
			})
		}

		t.Run("an infinite recursion should raise the error "+ErrMaximumSymbolicTestCallDepthReached.Error(), func(t *testing.T) {
			array1 := &Array{}
			array1.elements = []Value{array1}
			assert.PanicsWithError(t, ErrMaximumSymbolicTestCallDepthReached.Error(), func() {
				array1.Test(array1, RecTestCallState{})
			})

			array2 := &Array{}
			array2.generalElement = array2
			assert.PanicsWithError(t, ErrMaximumSymbolicTestCallDepthReached.Error(), func() {
				array2.Test(array2, RecTestCallState{})
			})
		})
	})

	t.Run("element() and IteratorElementValue()", func(t *testing.T) {
		cases := []struct {
			array   *Array
			element Value
		}{
			{
				&Array{elements: nil, generalElement: ANY},
				ANY,
			},
			{
				&Array{elements: nil, generalElement: ANY_INT},
				ANY_INT,
			},
			{
				&Array{elements: []Value{ANY_INT}, generalElement: nil},
				ANY_INT,
			},
			{
				&Array{elements: []Value{ANY_INT, ANY_INT}, generalElement: nil},
				ANY_INT,
			},
			{
				&Array{elements: []Value{INT_1, INT_2}, generalElement: nil},
				joinValues([]Value{INT_1, INT_2}),
			},
		}

		for _, testCase := range cases {
			t.Run(Stringify(testCase.array), func(t *testing.T) {
				element := testCase.array.element()
				if !assert.Equal(t, testCase.element, element) {
					return
				}
				assert.Equal(t, testCase.element, testCase.array.IteratorElementValue())
			})
		}

		t.Run("an infinite recursion should raise the error "+ErrMaximumSymbolicTestCallDepthReached.Error(), func(t *testing.T) {
			array1 := &Array{}
			array1.elements = []Value{array1}
			assert.PanicsWithError(t, ErrMaximumSymbolicTestCallDepthReached.Error(), func() {
				array1.Test(array1, RecTestCallState{})
			})

			array2 := &Array{}
			array2.generalElement = array2
			assert.PanicsWithError(t, ErrMaximumSymbolicTestCallDepthReached.Error(), func() {
				array2.Test(array2, RecTestCallState{})
			})
		})
	})

	t.Run("Contains()", func(t *testing.T) {
		intList := NewListOf(ANY_INT)
		assertMayContainButNotCertain(t, intList, ANY_SERIALIZABLE)
		assertMayContainButNotCertain(t, intList, ANY_INT)
		assertMayContainButNotCertain(t, intList, INT_1)
		assertMayContainButNotCertain(t, intList, INT_2)

		listWithConcreteValue := NewListOf(INT_1)
		assertContains(t, listWithConcreteValue, INT_1)
		assertMayContainButNotCertain(t, listWithConcreteValue, ANY_SERIALIZABLE)
		assertMayContainButNotCertain(t, listWithConcreteValue, ANY_INT)
		assertCannotPossiblyContain(t, listWithConcreteValue, INT_2)

		anyIntPair := NewList(ANY_INT, ANY_INT)
		assertMayContainButNotCertain(t, anyIntPair, ANY_SERIALIZABLE)
		assertMayContainButNotCertain(t, anyIntPair, ANY_INT)
		assertMayContainButNotCertain(t, anyIntPair, INT_1)
		assertMayContainButNotCertain(t, anyIntPair, INT_2)

		concretizableList := NewList(INT_1, INT_2)
		assertContains(t, concretizableList, INT_1)
		assertContains(t, concretizableList, INT_2)
		assertMayContainButNotCertain(t, concretizableList, ANY_SERIALIZABLE)
		assertMayContainButNotCertain(t, concretizableList, ANY_INT)
		assertCannotPossiblyContain(t, concretizableList, INT_3)
	})
}

func TestDictionary(t *testing.T) {

	t.Run("Test()", func(t *testing.T) {

		cases := []struct {
			dict1            *Dictionary
			dict2            *Dictionary
			oneTestTwoResult bool
		}{
			{
				&Dictionary{entries: nil},
				&Dictionary{entries: nil},
				true,
			},
			{
				&Dictionary{entries: map[string]Serializable{}},
				&Dictionary{entries: nil},
				false,
			},
			{
				&Dictionary{entries: nil},
				&Dictionary{entries: map[string]Serializable{}},
				true,
			},
			{
				&Dictionary{entries: map[string]Serializable{}, keys: map[string]Serializable{}},
				&Dictionary{entries: map[string]Serializable{}, keys: map[string]Serializable{}},
				true,
			},
			{
				&Dictionary{
					entries: map[string]Serializable{"./a": ANY_INT},
					keys:    map[string]Serializable{"./a": &Path{}},
				},
				&Dictionary{
					entries: map[string]Serializable{},
				},
				false,
			},
			{
				&Dictionary{
					entries: map[string]Serializable{},
				},
				&Dictionary{
					entries: map[string]Serializable{"./a": ANY_INT},
					keys:    map[string]Serializable{"./a": &Path{}},
				},
				false,
			},
			{
				&Dictionary{
					entries: map[string]Serializable{"./a": ANY_INT},
					keys:    map[string]Serializable{"./a": &Path{}},
				},
				&Dictionary{
					entries: map[string]Serializable{"./a": ANY_INT},
					keys:    map[string]Serializable{"./a": &Path{}},
				},
				true,
			},
			{
				&Dictionary{
					entries: map[string]Serializable{"./a": ANY_SERIALIZABLE},
					keys:    map[string]Serializable{"./a": &Path{}},
				},
				&Dictionary{
					entries: map[string]Serializable{"./a": ANY_INT},
					keys:    map[string]Serializable{"./a": &Path{}},
				},
				true,
			},
			{
				&Dictionary{
					entries: map[string]Serializable{"./a": ANY_INT},
					keys:    map[string]Serializable{"./a": &Path{}},
				},
				&Dictionary{
					entries: map[string]Serializable{"./a": ANY_SERIALIZABLE},
					keys:    map[string]Serializable{"./a": &Path{}},
				},
				false,
			},
		}

		for _, testCase := range cases {
			t.Run(t.Name()+"_"+fmt.Sprint(testCase.dict1, "_", testCase.dict2), func(t *testing.T) {
				assert.Equal(t, testCase.oneTestTwoResult, testCase.dict1.Test(testCase.dict2, RecTestCallState{}))
			})
		}

		t.Run("an infinite recursion should raise the error "+ErrMaximumSymbolicTestCallDepthReached.Error(), func(t *testing.T) {
			dict := &Dictionary{}

			dict.entries = map[string]Serializable{
				"./a": dict,
			}
			dict.keys = map[string]Serializable{
				"./a": NewPath("./a"),
			}

			assert.PanicsWithError(t, ErrMaximumSymbolicTestCallDepthReached.Error(), func() {
				dict.Test(dict, RecTestCallState{})
			})
		})
	})

}
