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
	set(i *Int, v SymbolicValue)
	SetSlice(start, end *Int, v Sequence)
}

type MutableLengthSequence interface {
	MutableSequence
	insertElement(v SymbolicValue, i *Int) *Error
	removePosition(i *Int) *Error
	//TODO: add removePositiontRange
	insertSequence(seq Sequence, i *Int) *Error
	appendSequence(seq Sequence) *Error
}
