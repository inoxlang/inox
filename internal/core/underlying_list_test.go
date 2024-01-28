package core

import (
	"testing"

	"github.com/inoxlang/inox/internal/utils"
	"github.com/stretchr/testify/assert"
)

func TestValueList(t *testing.T) {
	t.Parallel()

	newList := func(elems ...Serializable) underlyingList {
		return newValueList(elems...)
	}

	testUnderlyingList(t, underlyingTestSuiteParams[Serializable]{
		newList: newList,
		elemA:   Int(1),
		elemB:   Int(2),
		elemC:   Int(3),
		elemD:   Int(4),
		getCapacity: func(ul underlyingList) int {
			return len(ul.(*ValueList).elements)
		},
	})

}

func TestIntList(t *testing.T) {
	t.Parallel()

	newList := func(elems ...Int) underlyingList {
		return newIntList(elems...)
	}

	testUnderlyingList(t, underlyingTestSuiteParams[Int]{
		newList: newList,
		elemA:   Int(1),
		elemB:   Int(2),
		elemC:   Int(3),
		elemD:   Int(4),
		getCapacity: func(ul underlyingList) int {
			return len(ul.(*IntList).elements)
		},
	})
}

func TestStringList(t *testing.T) {
	t.Parallel()

	newList := func(elems ...StringLike) underlyingList {
		return newStringList(elems...)
	}

	testUnderlyingList(t, underlyingTestSuiteParams[StringLike]{
		newList: newList,
		elemA:   String("a"),
		elemB:   String("b"),
		elemC:   String("c"),
		elemD:   String("d"),
		getCapacity: func(ul underlyingList) int {
			return len(ul.(*StringList).elements)
		},
	})
}

func TestBoolList(t *testing.T) {
	t.Parallel()

	newList := func(elems ...Bool) underlyingList {
		return newBoolList(elems...)
	}

	testUnderlyingList(t, underlyingTestSuiteParams[Bool]{
		newList: newList,
		elemA:   True,
		elemB:   False,
		elemC:   True,
		elemD:   False,
	})
}

type underlyingTestSuiteParams[E Serializable] struct {
	newList                    func(elems ...E) underlyingList
	elemA, elemB, elemC, elemD E

	//if set auto shrinking will be tested
	getCapacity func(underlyingList) int
}

func testUnderlyingList[E Serializable](t *testing.T, params underlyingTestSuiteParams[E]) {
	ctx := NewContexWithEmptyState(ContextConfig{}, nil)
	defer ctx.CancelGracefully()

	getAllElements := func(list underlyingList) []Value {
		return IterateAllValuesOnly(ctx, list.Iterator(ctx, IteratorConfiguration{}))
	}

	newList := params.newList
	elemA := params.elemA
	elemB := params.elemB
	elemC := params.elemC
	elemD := params.elemD

	//TODO: add more cases

	t.Run("set", func(t *testing.T) {
		list := newList(elemA)
		list.set(ctx, 0, elemB)
		assert.Equal(t, []Value{elemB}, getAllElements(list))
	})

	t.Run("setSlice", func(t *testing.T) {
		t.Run("single element", func(t *testing.T) {
			ctx := NewContexWithEmptyState(ContextConfig{}, nil)
			list := newList(elemA)
			list.SetSlice(ctx, 0, 1, newList(elemB))
			assert.Equal(t, []Value{elemB}, getAllElements(list))
		})

		t.Run("several elements", func(t *testing.T) {
			ctx := NewContexWithEmptyState(ContextConfig{}, nil)
			list := newList(elemA, elemB)
			list.SetSlice(ctx, 0, 2, newList(elemC, elemD))
			assert.Equal(t, []Value{elemC, elemD}, getAllElements(list))
		})
	})

	t.Run("insertElement", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		list := newList(elemA)
		list.insertElement(ctx, elemB, 0)
		assert.Equal(t, []Value{elemB, elemA}, getAllElements(list))
	})

	t.Run("insertSequence", func(t *testing.T) {
		t.Run("at existing index", func(t *testing.T) {
			ctx := NewContexWithEmptyState(ContextConfig{}, nil)
			list := newList(elemA)
			list.insertSequence(ctx, newList(elemB, elemC), 0)
			assert.Equal(t, []Value{elemB, elemC, elemA}, getAllElements(list))
		})
		t.Run("at exclusive end", func(t *testing.T) {
			ctx := NewContexWithEmptyState(ContextConfig{}, nil)
			list := newList(elemA)
			list.insertSequence(ctx, newList(elemB, elemC), 1)
			assert.Equal(t, []Value{elemA, elemB, elemC}, getAllElements(list))
		})
	})

	t.Run("appendSequence", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		list := newList(elemA)
		list.appendSequence(ctx, newList(elemB, elemC))
		assert.Equal(t, []Value{elemA, elemB, elemC}, getAllElements(list))
	})

	t.Run("removePosition", func(t *testing.T) {
		ctx := NewContexWithEmptyState(ContextConfig{}, nil)
		defer ctx.CancelGracefully()

		list := newList(elemA, elemB, elemC, elemD)

		assert.Panics(t, func() {
			list.removePosition(ctx, 4)
		})

		list.removePosition(ctx, 0)
		assert.Equal(t, []Value{elemB, elemC, elemD}, getAllElements(list))

		list.removePosition(ctx, 1)
		assert.Equal(t, []Value{elemB, elemD}, getAllElements(list))

		list.removePosition(ctx, 1)
		assert.Equal(t, []Value{elemB}, getAllElements(list))

		list.removePosition(ctx, 0)
		assert.Equal(t, []Value{}, getAllElements(list))

		assert.Panics(t, func() {
			list.removePosition(ctx, 0)
		})
	})

	t.Run("removePositionRange", func(t *testing.T) {
		t.Run("single element", func(t *testing.T) {
			ctx := NewContexWithEmptyState(ContextConfig{}, nil)
			list := newList(elemA)
			list.removePositionRange(ctx, NewIntRange(0, 0))
			assert.Equal(t, []Value{}, getAllElements(list))
		})

		t.Run("several elements", func(t *testing.T) {
			ctx := NewContexWithEmptyState(ContextConfig{}, nil)
			list := newList(elemA, elemB)
			list.removePositionRange(ctx, NewIntRange(0, 1))
			assert.Equal(t, []Value{}, getAllElements(list))
		})

		if params.getCapacity != nil {
			providedElems := []E{elemA, elemB, elemC, elemD}

			elems := utils.Repeat(MIN_SHRINKABLE_LIST_LENGTH, func(index int) E {
				return providedElems[index%len(providedElems)]
			})

			t.Run("removing most elements in a single call: start > 0 and end < len - 1", func(t *testing.T) {
				list := newList(elems...)
				initialCapacity := params.getCapacity(list)
				start := int64(1)
				inclusiveEnd := int64(len(elems) - 2)
				finalExpectedLength := 2

				intRange := NewIntRange(start, inclusiveEnd)
				list.removePositionRange(ctx, intRange)

				assert.Equal(t, finalExpectedLength, list.Len())

				//the capacity should be reduced
				assert.LessOrEqual(t, params.getCapacity(list), initialCapacity/2)
			})

			t.Run("removing most elements in a single call: start = 0 and end < len - 1", func(t *testing.T) {
				list := newList(elems...)
				initialCapacity := params.getCapacity(list)
				start := int64(0)
				inclusiveEnd := int64(len(elems) - 2)
				finalExpectedLength := 1

				intRange := NewIntRange(start, inclusiveEnd)
				list.removePositionRange(ctx, intRange)

				assert.Equal(t, finalExpectedLength, list.Len())

				//the capacity should be reduced
				assert.LessOrEqual(t, params.getCapacity(list), initialCapacity/2)
			})

			t.Run("removing most elements in a single call: start > 0 and end == len - 1", func(t *testing.T) {
				list := newList(elems...)
				initialCapacity := params.getCapacity(list)
				start := int64(1)
				inclusiveEnd := int64(len(elems) - 1)
				finalExpectedLength := 1

				intRange := NewIntRange(start, inclusiveEnd)
				list.removePositionRange(ctx, intRange)

				assert.Equal(t, finalExpectedLength, list.Len())

				//the capacity should be reduced
				assert.LessOrEqual(t, params.getCapacity(list), initialCapacity/2)
			})
		}
	})
}
