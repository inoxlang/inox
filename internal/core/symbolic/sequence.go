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
	insertElement(ctx *Context, v SymbolicValue, i *Int) *Error
	removePosition(ctx *Context, i *Int) *Error
	//TODO: add removePositiontRange
	insertSequence(ctx *Context, seq Sequence, i *Int) *Error
	appendSequence(ctx *Context, seq Sequence) *Error
}
