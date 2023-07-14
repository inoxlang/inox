package core

var (
	_ = []MutableLengthSequence{&List{}, &ByteSlice{}, &RuneSlice{}}
)

type Sequence interface {
	Indexable
	slice(start, end int) Sequence
}

type MutableSequence interface {
	Sequence

	// after the modification, set should inform the watchers about a mutation of kind SetElemAtIndex (if the MutableSequence is watchable)
	set(ctx *Context, i int, v Value)

	SetSlice(ctx *Context, start, end int, v Sequence)
}

type MutableLengthSequence interface {
	MutableSequence

	// after the insertion, insertElement should inform the watchers about a mutation of kind InsertElemAtIndex (if the MutableLengthSequence is watchable)
	insertElement(ctx *Context, v Value, i Int)

	removePosition(ctx *Context, i Int)

	removePositionRange(ctx *Context, r IntRange)

	insertSequence(ctx *Context, seq Sequence, i Int)

	appendSequence(ctx *Context, seq Sequence)
}
