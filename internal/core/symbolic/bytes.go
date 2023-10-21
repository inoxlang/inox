package symbolic

import (
	pprint "github.com/inoxlang/inox/internal/pretty_print"
)

var (
	_ = []WrappedString{
		(*String)(nil), (*Identifier)(nil), (*Path)(nil), (*PathPattern)(nil), (*Host)(nil),
		(*HostPattern)(nil), (*URLPattern)(nil), (*CheckedString)(nil),
	}

	_ = []BytesLike{(*AnyBytesLike)(nil), (*ByteSlice)(nil), (*BytesConcatenation)(nil)}

	ANY_BYTES_LIKE   = &AnyBytesLike{}
	ANY_BYTE_SLICE   = &ByteSlice{}
	ANY_BYTE         = &Byte{}
	ANY_BYTES_CONCAT = &BytesConcatenation{}
)

// An WrappedBytes represents a symbolic WrappedBytes.
type WrappedBytes interface {
	Iterable
	underlyingBytes() *ByteSlice //TODO: change return type ? --> it isn't equivalent to concrete version
}

// A BytesLike represents a symbolic BytesLike.
type BytesLike interface {
	MutableSequence
	Iterable
	GetOrBuildBytes() *ByteSlice
}

// A ByteSlice represents a symbolic ByteSlice.
type ByteSlice struct {
	_ int
	SerializableMixin
	PseudoClonableMixin
}

func NewByteSlice() *ByteSlice {
	return &ByteSlice{}
}

func (s *ByteSlice) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	_, ok := v.(*ByteSlice)
	return ok
}

func (s *ByteSlice) IsConcretizable() bool {
	return false
}

func (s *ByteSlice) Concretize(ctx ConcreteContext) any {
	panic(ErrNotConcretizable)
}

func (s *ByteSlice) PrettyPrint(w PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("byte-slice")
}

func (s *ByteSlice) IteratorElementKey() Value {
	return ANY_INT
}

func (s *ByteSlice) IteratorElementValue() Value {
	return ANY_BYTE
}

func (s *ByteSlice) HasKnownLen() bool {
	return false
}

func (s *ByteSlice) KnownLen() int {
	return -1
}

func (s *ByteSlice) element() Value {
	return ANY_BYTE
}

func (*ByteSlice) elementAt(i int) Value {
	return ANY_BYTE
}

func (s *ByteSlice) WidestOfType() Value {
	return ANY_BYTE_SLICE
}

func (s *ByteSlice) Reader() *Reader {
	return &Reader{}
}

func (s *ByteSlice) GetOrBuildBytes() *ByteSlice {
	return s
}

func (s *ByteSlice) slice(start, end *Int) Sequence {
	return ANY_BYTE_SLICE
}

func (s *ByteSlice) set(ctx *Context, i *Int, v Value) {

}

func (s *ByteSlice) SetSlice(ctx *Context, start, end *Int, v Sequence) {

}

func (s *ByteSlice) insertElement(ctx *Context, v Value, i *Int) {

}

func (s *ByteSlice) removePosition(ctx *Context, i *Int) {

}

func (s *ByteSlice) insertSequence(ctx *Context, seq Sequence, i *Int) {
	if seq.HasKnownLen() && seq.KnownLen() == 0 {
		return
	}
	if _, ok := widenToSameStaticTypeInMultivalue(seq.element()).(*Byte); !ok {
		ctx.AddSymbolicGoFunctionError(fmtHasElementsOfType(s, ANY_BYTE))
	}
}

func (s *ByteSlice) appendSequence(ctx *Context, seq Sequence) {
	if seq.HasKnownLen() && seq.KnownLen() == 0 {
		return
	}
	if _, ok := widenToSameStaticTypeInMultivalue(seq.element()).(*Byte); !ok {
		ctx.AddSymbolicGoFunctionError(fmtHasElementsOfType(s, ANY_BYTE))
	}
}

func (s *ByteSlice) WatcherElement() Value {
	return ANY
}

// A Byte represents a symbolic Byte.
type Byte struct {
	_ int
	SerializableMixin
}

func (b *Byte) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	_, ok := v.(*Byte)

	return ok
}

func (b *Byte) Static() Pattern {
	return &TypePattern{val: ANY_BYTE}
}

func (b *Byte) PrettyPrint(w PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("byte")
}

func (b *Byte) WidestOfType() Value {
	return ANY_BYTE
}

func (b *Byte) Int64() (n *Int, signed bool) {
	return ANY_INT, false
}

// A AnyBytesLike represents a symbolic BytesLike we don't know the concrete type.
type AnyBytesLike struct {
	_ int
}

func (b *AnyBytesLike) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	_, ok := v.(BytesLike)
	return ok
}

func (b *AnyBytesLike) IteratorElementKey() Value {
	return ANY_INT
}

func (b *AnyBytesLike) IteratorElementValue() Value {
	return ANY_BYTE
}

func (b *AnyBytesLike) set(ctx *Context, i *Int, v Value) {

}

func (b *AnyBytesLike) HasKnownLen() bool {
	return false
}

func (b *AnyBytesLike) KnownLen() int {
	return -1
}

func (b *AnyBytesLike) element() Value {
	return ANY_BYTE
}

func (b *AnyBytesLike) elementAt(i int) Value {
	return ANY_BYTE
}

func (b *AnyBytesLike) slice(start, end *Int) Sequence {
	return ANY_BYTE_SLICE
}

func (c *AnyBytesLike) SetSlice(ctx *Context, start, end *Int, v Sequence) {

}

func (b *AnyBytesLike) PrettyPrint(w PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("bytes-like")
}

// func (b *AnyBytesLike) GetOrBuildString() *String {
// 	return &String{}
// }

func (b *AnyBytesLike) GetOrBuildBytes() *ByteSlice {
	return ANY_BYTE_SLICE
}

func (b *AnyBytesLike) WidestOfType() Value {
	return ANY_BYTES_LIKE
}

func (b *AnyBytesLike) Reader() *Reader {
	return &Reader{}
}

// A BytesConcatenation represents a symbolic BytesConcatenation.
type BytesConcatenation struct {
	_ int
}

func (c *BytesConcatenation) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	_, ok := v.(*BytesConcatenation)
	return ok
}

func (c *BytesConcatenation) IteratorElementKey() Value {
	return ANY_INT
}

func (c *BytesConcatenation) IteratorElementValue() Value {
	return ANY_BYTE
}

func (c *BytesConcatenation) set(ctx *Context, i *Int, v Value) {

}

func (c *BytesConcatenation) SetSlice(ctx *Context, start, end *Int, v Sequence) {

}

func (c *BytesConcatenation) HasKnownLen() bool {
	return false
}

func (s *BytesConcatenation) KnownLen() int {
	return -1
}

func (s *BytesConcatenation) element() Value {
	return ANY_BYTE
}

func (*BytesConcatenation) elementAt(i int) Value {
	return ANY_BYTE
}

func (c *BytesConcatenation) slice(start, end *Int) Sequence {
	return ANY_BYTE_SLICE
}

func (c *BytesConcatenation) PrettyPrint(w PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("bytes-concatenation")
}

// func (c *BytesConcatenation) GetOrBuildString() *String {
// 	return &String{}
// }

func (c *BytesConcatenation) WidestOfType() Value {
	return ANY_BYTES_CONCAT
}

func (c *BytesConcatenation) Reader() *Reader {
	return &Reader{}
}

func (c *BytesConcatenation) GetOrBuildBytes() *ByteSlice {
	return ANY_BYTE_SLICE
}
