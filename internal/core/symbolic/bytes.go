package symbolic

import (
	"bufio"

	pprint "github.com/inoxlang/inox/internal/pretty_print"
	"github.com/inoxlang/inox/internal/utils"
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
	underylingBytes() *ByteSlice //TODO: change return type ? --> it isn't equivalent to concrete version
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

func (s *ByteSlice) Test(v SymbolicValue) bool {
	_, ok := v.(*ByteSlice)
	return ok
}

func (s *ByteSlice) IsConcretizable() bool {
	return false
}

func (s *ByteSlice) Concretize() any {
	panic(ErrNotConcretizable)
}

func (s *ByteSlice) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%byte-slice")))
}

func (s *ByteSlice) IteratorElementKey() SymbolicValue {
	return ANY_INT
}

func (s *ByteSlice) IteratorElementValue() SymbolicValue {
	return ANY_BYTE
}

func (s *ByteSlice) HasKnownLen() bool {
	return false
}

func (s *ByteSlice) KnownLen() int {
	return -1
}

func (s *ByteSlice) element() SymbolicValue {
	return ANY_BYTE
}

func (*ByteSlice) elementAt(i int) SymbolicValue {
	return ANY_BYTE
}

func (s *ByteSlice) WidestOfType() SymbolicValue {
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

func (s *ByteSlice) set(ctx *Context, i *Int, v SymbolicValue) {

}

func (s *ByteSlice) SetSlice(ctx *Context, start, end *Int, v Sequence) {

}

func (s *ByteSlice) insertElement(ctx *Context, v SymbolicValue, i *Int) {

}
func (s *ByteSlice) removePosition(ctx *Context, i *Int) {

}
func (s *ByteSlice) insertSequence(ctx *Context, seq Sequence, i *Int) {

}
func (s *ByteSlice) appendSequence(ctx *Context, seq Sequence) {

}

func (s *ByteSlice) WatcherElement() SymbolicValue {
	return ANY
}

// A Byte represents a symbolic Byte.
type Byte struct {
	_ int
	SerializableMixin
}

func (b *Byte) Test(v SymbolicValue) bool {
	_, ok := v.(*Byte)

	return ok
}

func (b *Byte) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%byte")))
}

func (b *Byte) WidestOfType() SymbolicValue {
	return ANY_BYTE
}

func (b *Byte) Int64() (n *Int, signed bool) {
	return ANY_INT, false
}

// A AnyBytesLike represents a symbolic BytesLike we don't know the concrete type.
type AnyBytesLike struct {
	_ int
}

func (b *AnyBytesLike) Test(v SymbolicValue) bool {
	_, ok := v.(BytesLike)
	return ok
}

func (b *AnyBytesLike) IteratorElementKey() SymbolicValue {
	return ANY_INT
}

func (b *AnyBytesLike) IteratorElementValue() SymbolicValue {
	return ANY_BYTE
}

func (b *AnyBytesLike) set(ctx *Context, i *Int, v SymbolicValue) {

}

func (b *AnyBytesLike) HasKnownLen() bool {
	return false
}

func (b *AnyBytesLike) KnownLen() int {
	return -1
}

func (b *AnyBytesLike) element() SymbolicValue {
	return ANY_BYTE
}

func (b *AnyBytesLike) elementAt(i int) SymbolicValue {
	return ANY_BYTE
}

func (b *AnyBytesLike) slice(start, end *Int) Sequence {
	return ANY_BYTE_SLICE
}

func (c *AnyBytesLike) SetSlice(ctx *Context, start, end *Int, v Sequence) {

}

func (b *AnyBytesLike) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%bytes-like")))
}

// func (b *AnyBytesLike) GetOrBuildString() *String {
// 	return &String{}
// }

func (b *AnyBytesLike) GetOrBuildBytes() *ByteSlice {
	return ANY_BYTE_SLICE
}

func (b *AnyBytesLike) WidestOfType() SymbolicValue {
	return ANY_BYTES_LIKE
}

func (b *AnyBytesLike) Reader() *Reader {
	return &Reader{}
}

// A BytesConcatenation represents a symbolic BytesConcatenation.
type BytesConcatenation struct {
	_ int
}

func (c *BytesConcatenation) Test(v SymbolicValue) bool {
	_, ok := v.(*BytesConcatenation)
	return ok
}

func (c *BytesConcatenation) IteratorElementKey() SymbolicValue {
	return ANY_INT
}

func (c *BytesConcatenation) IteratorElementValue() SymbolicValue {
	return ANY_BYTE
}

func (c *BytesConcatenation) set(ctx *Context, i *Int, v SymbolicValue) {

}

func (c *BytesConcatenation) SetSlice(ctx *Context, start, end *Int, v Sequence) {

}

func (c *BytesConcatenation) HasKnownLen() bool {
	return false
}

func (s *BytesConcatenation) KnownLen() int {
	return -1
}

func (s *BytesConcatenation) element() SymbolicValue {
	return ANY_BYTE
}

func (*BytesConcatenation) elementAt(i int) SymbolicValue {
	return ANY_BYTE
}

func (c *BytesConcatenation) slice(start, end *Int) Sequence {
	return ANY_BYTE_SLICE
}

func (c *BytesConcatenation) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%bytes-concatenation")))
}

// func (c *BytesConcatenation) GetOrBuildString() *String {
// 	return &String{}
// }

func (c *BytesConcatenation) WidestOfType() SymbolicValue {
	return ANY_BYTES_CONCAT
}

func (c *BytesConcatenation) Reader() *Reader {
	return &Reader{}
}

func (c *BytesConcatenation) GetOrBuildBytes() *ByteSlice {
	return ANY_BYTE_SLICE
}
