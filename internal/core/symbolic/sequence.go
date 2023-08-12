package symbolic

var (
	_ = []Sequence{(*String)(nil), (*Tuple)(nil)}
	_ = []MutableLengthSequence{(*List)(nil), (*ByteSlice)(nil), (*RuneSlice)(nil)}
)

type Sequence interface {
	Indexable
	slice(start, end *Int) Sequence
}

type MutableSequence interface {
	Sequence
	set(ctx *Context, i *Int, v SymbolicValue)
	SetSlice(ctx *Context, start, end *Int, v Sequence)
}

type MutableLengthSequence interface {
	MutableSequence
	insertElement(ctx *Context, v SymbolicValue, i *Int)
	removePosition(ctx *Context, i *Int)
	//TODO: add removePositionRange
	insertSequence(ctx *Context, seq Sequence, i *Int)
	appendSequence(ctx *Context, seq Sequence)
}
