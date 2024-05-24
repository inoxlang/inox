package symbolic

import (
	"context"
	"fmt"
	"testing"

	"github.com/inoxlang/inox/internal/ast"
	"github.com/stretchr/testify/assert"
)

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
			state.consumeSymbolicGoFunctionErrors(func(msg string, optionalLocation ast.Node) {
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

			state.consumeSymbolicGoFunctionErrors(func(msg string, optionalLocation ast.Node) {
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

			state.consumeSymbolicGoFunctionErrors(func(msg string, optionalLocation ast.Node) {
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

			state.consumeSymbolicGoFunctionErrors(func(msg string, optionalLocation ast.Node) {
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
				element := testCase.array.Element()
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
