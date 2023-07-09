package core

import "github.com/bits-and-blooms/bitset"

type underylingList interface {
	Serializable
	MutableLengthSequence
	Iterable
	ContainsSimple(ctx *Context, v Serializable) bool
	append(ctx *Context, values ...Serializable)
}

// ValueList implements underylingList
type ValueList struct {
	elements     []Serializable
	constraintId ConstraintId
}

func NewWrappedValueList(elements ...Serializable) *List {
	return newList(&ValueList{elements: elements})
}

func NewWrappedValueListFrom(elements []Serializable) *List {
	return newList(&ValueList{elements: elements})
}

func newValueList(elements ...Serializable) *ValueList {
	return &ValueList{elements: elements}
}

func (list *ValueList) ContainsSimple(ctx *Context, v Serializable) bool {
	if !IsSimpleInoxVal(v) {
		panic("only simple values are expected")
	}

	for _, e := range list.elements {
		if v.Equal(nil, e, map[uintptr]uintptr{}, 0) {
			return true
		}
	}
	return false
}

func (list *ValueList) set(ctx *Context, i int, v Value) {
	list.elements[i] = v.(Serializable)
}

func (list *ValueList) setSlice(ctx *Context, start, end int, v Value) {
	i := start
	it := v.(*List).Iterator(ctx, IteratorConfiguration{})

	for it.Next(ctx) {
		e := it.Value(ctx)
		list.elements[i] = e.(Serializable)
		i++
	}
}

func (list *ValueList) slice(start, end int) Sequence {
	sliceCopy := make([]Serializable, end-start)
	copy(sliceCopy, list.elements[start:end])

	return &List{underylingList: &ValueList{elements: sliceCopy}}
}

func (list *ValueList) Len() int {
	return len(list.elements)
}

func (list *ValueList) At(ctx *Context, i int) Value {
	return list.elements[i]
}

func (list *ValueList) append(ctx *Context, values ...Serializable) {
	list.elements = append(list.elements, values...)
}

func (l *ValueList) insertElement(ctx *Context, v Value, i Int) {
	length := Int(l.Len())
	if i < 0 || i > length {
		panic(ErrInsertionIndexOutOfRange)
	}
	if i == length {
		l.elements = append(l.elements, v.(Serializable))
	} else {
		l.elements = append(l.elements, nil)
		copy(l.elements[i+1:], l.elements[i:])
		l.elements[i] = v.(Serializable)
	}
}

func (l *ValueList) removePosition(ctx *Context, i Int) {
	panic(ErrNotImplementedYet)
	// if i <= len(l.Elements)-1 {
	// 	copy(l.Elements[i:], l.Elements[i+1:])
	// }
	// l.Elements = l.Elements[:len(l.Elements)-1]
}

func (l *ValueList) removePositionRange(ctx *Context, r IntRange) {
	panic(ErrNotImplementedYet)
}

func (l *ValueList) insertSequence(ctx *Context, seq Sequence, i Int) {
	panic(ErrNotImplementedYet)
}

func (l *ValueList) appendSequence(ctx *Context, seq Sequence) {
	panic(ErrNotImplementedYet)
}

// IntList implements underylingList
type IntList struct {
	Elements     []Int
	constraintId ConstraintId
}

func NewWrappedIntList(elements ...Int) *List {
	return &List{underylingList: newIntList(elements...)}
}

func NewWrappedIntListFrom(elements []Int) *List {
	return &List{underylingList: &IntList{Elements: elements}}
}

func newIntList(elements ...Int) *IntList {
	return &IntList{Elements: elements}
}

func (list *IntList) ContainsSimple(ctx *Context, v Serializable) bool {
	if !IsSimpleInoxVal(v) {
		panic("only simple values are expected")
	}

	integer, ok := v.(Int)
	if !ok {
		return false
	}

	for _, n := range list.Elements {
		if n == integer {
			return true
		}
	}
	return false
}

func (list *IntList) set(ctx *Context, i int, v Value) {
	list.Elements[i] = v.(Int)
}

func (list *IntList) setSlice(ctx *Context, start, end int, v Value) {
	i := start
	it := v.(*List).Iterator(ctx, IteratorConfiguration{})

	for it.Next(ctx) {
		e := it.Value(ctx)
		list.Elements[i] = e.(Int)
		i++
	}
}

func (list *IntList) slice(start, end int) Sequence {
	sliceCopy := make([]Int, end-start)
	copy(sliceCopy, list.Elements[start:end])

	return &List{underylingList: &IntList{Elements: sliceCopy}}
}

func (list *IntList) Len() int {
	return len(list.Elements)
}

func (list *IntList) At(ctx *Context, i int) Value {
	return list.Elements[i]
}

func (list *IntList) append(ctx *Context, values ...Serializable) {
	for _, val := range values {
		list.Elements = append(list.Elements, val.(Int))
	}
}

func (l *IntList) insertElement(ctx *Context, v Value, i Int) {
	length := Int(l.Len())
	if i < 0 || i > length {
		panic(ErrInsertionIndexOutOfRange)
	}
	if i == length {
		l.Elements = append(l.Elements, v.(Int))
	} else {
		l.Elements = append(l.Elements, 0)
		copy(l.Elements[i+1:], l.Elements[i:])
		l.Elements[i] = v.(Int)
	}
}

func (l *IntList) removePosition(ctx *Context, i Int) {
	panic(ErrNotImplementedYet)
	// if i <= len(l.Elements)-1 {
	// 	copy(l.Elements[i:], l.Elements[i+1:])
	// }
	// l.Elements = l.Elements[:len(l.Elements)-1]
}

func (l *IntList) removePositionRange(ctx *Context, r IntRange) {
	panic(ErrNotImplementedYet)
}

func (l *IntList) insertSequence(ctx *Context, seq Sequence, i Int) {
	panic(ErrNotImplementedYet)
}

func (l *IntList) appendSequence(ctx *Context, seq Sequence) {
	panic(ErrNotImplementedYet)
}

// StringList implements underylingList
type StringList struct {
	elements     []StringLike
	constraintId ConstraintId
}

func NewWrappedStringList(elements ...StringLike) *List {
	return &List{underylingList: newStringList(elements...)}
}

func NewWrappedStringListFrom(elements []StringLike) *List {
	return &List{underylingList: &StringList{elements: elements}}
}

func newStringList(elements ...StringLike) *StringList {
	return &StringList{elements: elements}
}

func (list *StringList) ContainsSimple(ctx *Context, v Serializable) bool {
	if !IsSimpleInoxVal(v) {
		panic("only simple values are expected")
	}

	str, ok := v.(StringLike)
	if !ok {
		return false
	}

	for _, n := range list.elements {
		if n.GetOrBuildString() == str.GetOrBuildString() {
			return true
		}
	}
	return false
}

func (list *StringList) set(ctx *Context, i int, v Value) {
	list.elements[i] = v.(StringLike)
}

func (list *StringList) setSlice(ctx *Context, start, end int, v Value) {
	i := start
	it := v.(*List).Iterator(ctx, IteratorConfiguration{})

	for it.Next(ctx) {
		e := it.Value(ctx)
		list.elements[i] = e.(StringLike)
		i++
	}
}

func (list *StringList) slice(start, end int) Sequence {
	sliceCopy := make([]StringLike, end-start)
	copy(sliceCopy, list.elements[start:end])

	return &List{underylingList: &StringList{elements: sliceCopy}}
}

func (list *StringList) Len() int {
	return len(list.elements)
}

func (list *StringList) At(ctx *Context, i int) Value {
	return list.elements[i]
}

func (list *StringList) append(ctx *Context, values ...Serializable) {
	for _, val := range values {
		list.elements = append(list.elements, val.(StringLike))
	}
}

func (l *StringList) insertElement(ctx *Context, v Value, i Int) {
	length := Int(l.Len())
	if i < 0 || i > length {
		panic(ErrInsertionIndexOutOfRange)
	}
	if i == length {
		l.elements = append(l.elements, v.(StringLike))
	} else {
		l.elements = append(l.elements, nil)
		copy(l.elements[i+1:], l.elements[i:])
		l.elements[i] = v.(StringLike)
	}
}

func (l *StringList) removePosition(ctx *Context, i Int) {
	panic(ErrNotImplementedYet)
	// if i <= len(l.Elements)-1 {
	// 	copy(l.Elements[i:], l.Elements[i+1:])
	// }
	// l.Elements = l.Elements[:len(l.Elements)-1]
}

func (l *StringList) removePositionRange(ctx *Context, r IntRange) {
	panic(ErrNotImplementedYet)
}

func (l *StringList) insertSequence(ctx *Context, seq Sequence, i Int) {
	panic(ErrNotImplementedYet)
}

func (l *StringList) appendSequence(ctx *Context, seq Sequence) {
	panic(ErrNotImplementedYet)
}

// BoolList implements underylingList
type BoolList struct {
	elements     *bitset.BitSet
	constraintId ConstraintId
}

func NewWrappedBoolList(elements ...Bool) *List {
	return &List{underylingList: newBoolList(elements...)}
}

func newBoolList(elements ...Bool) *BoolList {
	bitset := bitset.New(uint(len(elements)))
	for i, boolean := range elements {
		if boolean {
			bitset.Set(uint(i))
		}
	}
	return &BoolList{elements: bitset}
}

func (list *BoolList) ContainsSimple(ctx *Context, v Serializable) bool {
	if !IsSimpleInoxVal(v) {
		panic("only booleans are expected")
	}

	boolean, ok := v.(Bool)
	if !ok {
		return false
	}

	if boolean {
		_, ok := list.elements.NextSet(0)
		return ok
	}

	_, ok = list.elements.NextClear(0)
	return ok
}

func (list *BoolList) set(ctx *Context, i int, v Value) {
	boolean := v.(Bool)
	list.elements.SetTo(uint(i), bool(boolean))
}

func (list *BoolList) setSlice(ctx *Context, start, end int, v Value) {
	i := start
	it := v.(*List).Iterator(ctx, IteratorConfiguration{})

	for it.Next(ctx) {
		e := it.Value(ctx)
		boolean := e.(Bool)
		list.elements.SetTo(uint(i), bool(boolean))
		i++
	}
}

func (list *BoolList) slice(start, end int) Sequence {
	panic(ErrNotImplementedYet)
}

func (list *BoolList) Len() int {
	return int(list.elements.Len())
}

func (list *BoolList) BoolAt(i int) bool {
	return list.elements.Test(uint(i))
}

func (list *BoolList) At(ctx *Context, i int) Value {
	panic(ErrNotImplementedYet)
}

func (list *BoolList) append(ctx *Context, values ...Serializable) {
	panic(ErrNotImplementedYet)
}

func (l *BoolList) insertElement(ctx *Context, v Value, i Int) {
	panic(ErrNotImplementedYet)
}

func (l *BoolList) removePosition(ctx *Context, i Int) {
	panic(ErrNotImplementedYet)
	// if i <= len(l.Elements)-1 {
	// 	copy(l.Elements[i:], l.Elements[i+1:])
	// }
	// l.Elements = l.Elements[:len(l.Elements)-1]
}

func (l *BoolList) removePositionRange(ctx *Context, r IntRange) {
	panic(ErrNotImplementedYet)
}

func (l *BoolList) insertSequence(ctx *Context, seq Sequence, i Int) {
	panic(ErrNotImplementedYet)
}

func (l *BoolList) appendSequence(ctx *Context, seq Sequence) {
	panic(ErrNotImplementedYet)
}
