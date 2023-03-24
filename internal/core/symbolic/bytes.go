package internal

var (
	_ = []WrappedString{
		&String{}, &Identifier{}, &Path{}, &PathPattern{}, &Host{},
		&HostPattern{}, &URLPattern{}, &CheckedString{},
	}

	_ = []BytesLike{
		&ByteSlice{}, &BytesConcatenation{},
	}

	ANY_BYTES_LIKE = &AnyBytesLike{}
)

// An WrappedBytes represents a symbolic WrappedBytes.
type WrappedBytes interface {
	underylingBytes() *ByteSlice //TODO: change return type ? --> it isn't equivalent to concrete version
}

// A BytesLike represents a symbolic BytesLike.
type BytesLike interface {
	SymbolicValue
	GetOrBuildBytes() *ByteSlice
}

// A ByteSlice represents a symbolic ByteSlice.
type ByteSlice struct {
	_ int
}

func (s *ByteSlice) Test(v SymbolicValue) bool {
	_, ok := v.(*ByteSlice)
	return ok
}

func (s *ByteSlice) Widen() (SymbolicValue, bool) {
	return nil, false
}

func (s *ByteSlice) IsWidenable() bool {
	return false
}

func (s *ByteSlice) String() string {
	return "byte-slice"
}

func (s *ByteSlice) HasKnownLen() bool {
	return false
}

func (s *ByteSlice) knownLen() int {
	return -1
}

func (s *ByteSlice) element() SymbolicValue {
	return &Byte{}
}

func (*ByteSlice) elementAt(i int) SymbolicValue {
	return &Byte{}
}

func (s *ByteSlice) WidestOfType() SymbolicValue {
	return &ByteSlice{}
}

func (s *ByteSlice) Reader() *Reader {
	return &Reader{}
}

func (s *ByteSlice) GetOrBuildBytes() *ByteSlice {
	return s
}

func (s *ByteSlice) slice(start, end *Int) Sequence {
	return &ByteSlice{}
}

func (s *ByteSlice) set(i *Int, v SymbolicValue) {

}
func (s *ByteSlice) setSlice(start, end *Int, v SymbolicValue) {

}

func (s *ByteSlice) insertElement(v SymbolicValue, i *Int) *Error {
	return nil
}
func (s *ByteSlice) removePosition(i *Int) *Error {
	return nil
}
func (s *ByteSlice) insertSequence(seq Sequence, i *Int) *Error {
	return nil
}
func (s *ByteSlice) appendSequence(seq Sequence) *Error {
	return nil
}

// A Byte represents a symbolic Byte.
type Byte struct {
	_ int
}

func (b *Byte) Test(v SymbolicValue) bool {
	_, ok := v.(*Byte)

	return ok
}

func (b *Byte) Widen() (SymbolicValue, bool) {
	return nil, false
}

func (b *Byte) IsWidenable() bool {
	return false
}

func (b *Byte) String() string {
	return "byte"
}

func (b *Byte) WidestOfType() SymbolicValue {
	return &Byte{}
}

func (b *Byte) Int64() (n *Int, signed bool) {
	return &Int{}, false
}

// A AnyBytesLike represents a symbolic BytesLike we don't know the concrete type.
type AnyBytesLike struct {
	_ int
}

func (b *AnyBytesLike) Test(v SymbolicValue) bool {
	_, ok := v.(BytesLike)
	return ok
}

func (b *AnyBytesLike) Widen() (SymbolicValue, bool) {
	return nil, false
}

func (b *AnyBytesLike) IsWidenable() bool {
	return false
}

func (b *AnyBytesLike) String() string {
	return "bytes-like"
}

func (b *AnyBytesLike) HasKnownLen() bool {
	return false
}

func (b *AnyBytesLike) knownLen() int {
	return -1
}

func (b *AnyBytesLike) element() SymbolicValue {
	return &Byte{}
}

// func (b *AnyBytesLike) GetOrBuildString() *String {
// 	return &String{}
// }

func (b *AnyBytesLike) GetOrBuildBytes() *ByteSlice {
	return &ByteSlice{}
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

func (c *BytesConcatenation) Widen() (SymbolicValue, bool) {
	return nil, false
}

func (c *BytesConcatenation) IsWidenable() bool {
	return false
}

func (c *BytesConcatenation) String() string {
	return "bytes-concatenation"
}

func (c *BytesConcatenation) HasKnownLen() bool {
	return false
}

func (c *BytesConcatenation) knownLen() int {
	return -1
}

func (c *BytesConcatenation) element() SymbolicValue {
	return &Rune{}
}

// func (c *BytesConcatenation) GetOrBuildString() *String {
// 	return &String{}
// }

func (c *BytesConcatenation) WidestOfType() SymbolicValue {
	return &BytesConcatenation{}
}

func (c *BytesConcatenation) Reader() *Reader {
	return &Reader{}
}

func (c *BytesConcatenation) GetOrBuildBytes() *ByteSlice {
	return &ByteSlice{}
}
