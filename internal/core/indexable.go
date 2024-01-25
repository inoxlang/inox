package core

import (
	"strconv"
	"sync"

	"slices"

	"github.com/inoxlang/inox/internal/core/symbolic"
)

type Indexable interface {
	Iterable

	// At should panic if the index is out of bounds.
	At(ctx *Context, i int) Value

	Len() int
}

type Array []Value

func NewArrayFrom(elements ...Value) *Array {
	if elements == nil {
		elements = []Value{}
	}
	array := Array(elements)
	return &array
}

func NewArray(ctx *Context, elements ...Value) *Array {
	return NewArrayFrom(elements...)
}

func (a *Array) At(ctx *Context, i int) Value {
	return (*a)[i]
}

func (a *Array) Len() int {
	return len(*a)
}

func (a *Array) slice(start int, end int) Sequence {
	slice := (*a)[start:end]
	return &slice
}

// A List represents a sequence of elements, List implements Value.
// The elements are stored in an underlyingList that is suited for the number and kind of elements, for example
// if the elements are all integers the underlying list will (ideally) be an *IntList.
type List struct {
	underlyingList
	elemType Pattern

	lock                     sync.Mutex // exclusive access for initializing .watchers & .mutationCallbacks
	mutationCallbacks        *MutationCallbacks
	watchers                 *ValueWatchers
	watchingDepth            WatchingDepth
	elementMutationCallbacks []CallbackHandle
}

func newList(underlyingList underlyingList) *List {
	return &List{underlyingList: underlyingList}
}

func WrapUnderlyingList(l underlyingList) *List {
	return &List{underlyingList: l}
}

// the caller can modify the result.
func (list *List) GetOrBuildElements(ctx *Context) []Serializable {
	entries := IterateAll(ctx, list.Iterator(ctx, IteratorConfiguration{}))

	values := make([]Serializable, len(entries))
	for i, e := range entries {
		values[i] = e[1].(Serializable)
	}
	return values
}

func (l *List) Prop(ctx *Context, name string) Value {
	switch name {
	case "append":
		return WrapGoMethod(l.append)
	case "dequeue":
		return WrapGoMethod(l.Dequeue)
	case "pop":
		return WrapGoMethod(l.Pop)
	case "sorted":
		return WrapGoMethod(l.Sorted)
	case "sort_by":
		return WrapGoMethod(l.SortBy)
	case "len":
		return Int(l.Len())
	default:
		panic(FormatErrPropertyDoesNotExist(name, l))
	}
}

func (*List) SetProp(ctx *Context, name string, value Value) error {
	return ErrCannotSetProp
}

func (*List) PropertyNames(ctx *Context) []string {
	return symbolic.LIST_PROPNAMES
}

func (l *List) set(ctx *Context, i int, v Value) {
	prevElement := l.underlyingList.At(ctx, i)
	l.underlyingList.set(ctx, i, v)

	if l.elementMutationCallbacks != nil {
		l.removeElementMutationCallbackNoLock(ctx, i, prevElement.(Serializable))
		l.addElementMutationCallbackNoLock(ctx, i, v)
	}

	mutation := NewSetElemAtIndexMutation(ctx, i, v.(Serializable), ShallowWatching, Path("/"+strconv.Itoa(i)))

	//inform watchers & microtasks about the update
	l.watchers.InformAboutAsync(ctx, mutation, mutation.Depth, true)
	l.mutationCallbacks.CallMicrotasks(ctx, mutation)
}

func (l *List) SetSlice(ctx *Context, start, end int, seq Sequence) {
	if l.elementMutationCallbacks != nil {
		for i := start; i < end; i++ {
			prevElement := l.underlyingList.At(ctx, i)
			l.removeElementMutationCallbackNoLock(ctx, i, prevElement.(Serializable))
		}
	}

	l.underlyingList.SetSlice(ctx, start, end, seq)

	if l.elementMutationCallbacks != nil {
		for i := start; i < end; i++ {
			l.addElementMutationCallbackNoLock(ctx, i, l.underlyingList.At(ctx, i))
		}
	}

	path := Path("/" + strconv.Itoa(int(start)) + ".." + strconv.Itoa(int(end-1)))
	mutation := NewSetSliceAtRangeMutation(ctx, NewIncludedEndIntRange(int64(start), int64(end-1)), seq.(Serializable), ShallowWatching, path)

	l.mutationCallbacks.CallMicrotasks(ctx, mutation)
	l.watchers.InformAboutAsync(ctx, mutation, ShallowWatching, true)
}

func (l *List) insertElement(ctx *Context, v Value, i Int) {
	l.underlyingList.insertElement(ctx, v, i)

	if l.elementMutationCallbacks != nil {
		l.elementMutationCallbacks = slices.Insert(l.elementMutationCallbacks, int(i), FIRST_VALID_CALLBACK_HANDLE-1)
		l.addElementMutationCallbackNoLock(ctx, int(i), v)
	}

	mutation := NewInsertElemAtIndexMutation(ctx, int(i), v.(Serializable), ShallowWatching, Path("/"+strconv.Itoa(int(i))))

	//inform watchers & microtasks about the update
	l.watchers.InformAboutAsync(ctx, mutation, mutation.Depth, true)
	l.mutationCallbacks.CallMicrotasks(ctx, mutation)
}

func (l *List) insertSequence(ctx *Context, seq Sequence, i Int) {
	l.underlyingList.insertSequence(ctx, seq, i)

	if l.elementMutationCallbacks != nil {
		seqLen := seq.Len()
		l.elementMutationCallbacks = slices.Insert(l.elementMutationCallbacks, int(i), makeMutationCallbackHandles(seqLen)...)

		seqIndex := 0
		for index := i; index < i+Int(seqLen); index++ {
			l.addElementMutationCallbackNoLock(ctx, int(index), seq.At(ctx, seqIndex))
			seqIndex++
		}
	}

	mutation := NewInsertSequenceAtIndexMutation(ctx, int(i), seq, ShallowWatching, Path("/"+strconv.Itoa(int(i))))

	//inform watchers & microtasks about the update
	l.watchers.InformAboutAsync(ctx, mutation, mutation.Depth, true)
	l.mutationCallbacks.CallMicrotasks(ctx, mutation)
}

func (l *List) appendSequence(ctx *Context, seq Sequence) {
	l.insertSequence(ctx, seq, Int(l.Len()))
}

func (l *List) append(ctx *Context, elements ...Serializable) {
	index := l.Len()
	l.underlyingList.append(ctx, elements...)

	seq := NewWrappedValueList(elements...)

	if l.elementMutationCallbacks != nil {
		seqLen := seq.Len()
		l.elementMutationCallbacks = slices.Insert(l.elementMutationCallbacks, index, makeMutationCallbackHandles(seqLen)...)

		for i := index; i < index+len(elements); i++ {
			l.addElementMutationCallbackNoLock(ctx, int(i), seq.At(ctx, int(i-index)))
		}
	}

	mutation := NewInsertSequenceAtIndexMutation(ctx, index, seq, ShallowWatching, Path("/"+strconv.Itoa(index)))

	//inform watchers & microtasks about the update
	l.watchers.InformAboutAsync(ctx, mutation, mutation.Depth, true)
	l.mutationCallbacks.CallMicrotasks(ctx, mutation)
}

func (l *List) removePosition(ctx *Context, i Int) {
	l.underlyingList.removePosition(ctx, i)

	if l.elementMutationCallbacks != nil {
		l.removeElementMutationCallbackNoLock(ctx, int(i), l.underlyingList.At(ctx, int(i)).(Serializable))
		l.elementMutationCallbacks = slices.Replace(l.elementMutationCallbacks, int(i), int(i+1))
	}

	mutation := NewRemovePositionMutation(ctx, int(i), ShallowWatching, Path("/"+strconv.Itoa(int(i))))

	//inform watchers & microtasks about the update
	l.watchers.InformAboutAsync(ctx, mutation, mutation.Depth, true)
	l.mutationCallbacks.CallMicrotasks(ctx, mutation)
}

func (l *List) Dequeue(ctx *Context) Serializable {
	if l.Len() == 0 {
		panic(ErrCannotDequeueFromEmptyList)
	}
	elem := l.At(ctx, 0)
	l.removePosition(ctx, 0)
	return elem.(Serializable)
}

func (l *List) Pop(ctx *Context) Serializable {
	lastIndex := l.Len() - 1
	if lastIndex < 0 {
		panic(ErrCannotPopFromEmptyList)
	}
	elem := l.At(ctx, lastIndex)
	l.removePosition(ctx, Int(lastIndex))
	return elem.(Serializable)
}

func (l *List) removePositionRange(ctx *Context, r IntRange) {
	l.underlyingList.removePositionRange(ctx, r)

	if l.elementMutationCallbacks != nil {
		for index := int(r.start); index < int(r.end); index++ {
			l.removeElementMutationCallbackNoLock(ctx, index, l.underlyingList.At(ctx, index).(Serializable))
		}

		l.elementMutationCallbacks = slices.Replace(l.elementMutationCallbacks, int(r.start), int(r.end))
	}

	path := Path("/" + strconv.Itoa(int(r.KnownStart())) + ".." + strconv.Itoa(int(r.InclusiveEnd())))
	mutation := NewRemovePositionRangeMutation(ctx, r, ShallowWatching, path)

	//inform watchers & microtasks about the update
	l.watchers.InformAboutAsync(ctx, mutation, mutation.Depth, true)
	l.mutationCallbacks.CallMicrotasks(ctx, mutation)
}
