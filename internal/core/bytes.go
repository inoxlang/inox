package internal

import (
	"errors"
	"fmt"
)

var (
	_ = []WrappedBytes{&ByteSlice{}}
	_ = []BytesLike{&ByteSlice{}, &BytesConcatenation{}}
)

// A WrappedBytes represents a value that wraps a byte slice []byte.
type WrappedBytes interface {
	Readable
	//the returned bytes should NOT be modified
	UnderlyingBytes() []byte
}

// A BytesLike represents an abstract byte slice, it should behave exactly like a regular ByteSlice and have the same pseudo properties.
type BytesLike interface {
	MutableSequence
	GetOrBuildBytes() *ByteSlice
	Mutable() bool
}

// ByteSlice implements Value.
type ByteSlice struct {
	Bytes         []byte
	IsDataMutable bool
	contentType   Mimetype
	constraintId  ConstraintId
}

func NewByteSlice(bytes []byte, mutable bool, contentType Mimetype) *ByteSlice {
	//TODO: check content type
	if contentType != "" && mutable {
		panic(errors.New("attempt to create a mutable byte slice with specific content type"))
	}
	return &ByteSlice{Bytes: bytes, IsDataMutable: mutable, contentType: contentType}
}

func (slice *ByteSlice) UnderlyingBytes() []byte {
	return slice.Bytes
}

func (slice *ByteSlice) GetOrBuildBytes() *ByteSlice {
	return slice
}

func (slice *ByteSlice) Mutable() bool {
	return slice.IsDataMutable
}

func (slice *ByteSlice) ContentType() Mimetype {
	if slice.contentType == "" {
		return APP_OCTET_STREAM_CTYPE
	}
	return slice.contentType
}

func (slice *ByteSlice) slice(start, end int) Sequence {
	sliceCopy := make([]byte, end-start)
	copy(sliceCopy, slice.Bytes[start:end])

	return &ByteSlice{Bytes: sliceCopy, IsDataMutable: slice.IsDataMutable}
}

func (slice *ByteSlice) Len() int {
	return len(slice.Bytes)
}

func (slice *ByteSlice) At(ctx *Context, i int) Value {
	return Byte(slice.Bytes[i])
}

func (slice *ByteSlice) set(ctx *Context, i int, v Value) {
	if !slice.IsDataMutable {
		panic(fmt.Errorf("attempt to write a readonly byte slice"))
	}
	slice.Bytes[i] = byte(v.(Byte))
}

func (slice *ByteSlice) setSlice(ctx *Context, start, end int, v Value) {
	if !slice.IsDataMutable {
		panic(fmt.Errorf("attempt to write a readonly byte slice"))
	}
	i := start

	for _, e := range v.(*ByteSlice).Bytes {
		slice.Bytes[i] = e
		i++
	}
}

func (s *ByteSlice) insertElement(ctx *Context, v Value, i Int) {
	panic(ErrNotImplementedYet)
}

func (s *ByteSlice) removePosition(ctx *Context, i Int) {
	panic(ErrNotImplementedYet)
}

func (s *ByteSlice) removePositionRange(ctx *Context, r IntRange) {
	panic(ErrNotImplementedYet)
}

func (s *ByteSlice) insertSequence(ctx *Context, seq Sequence, i Int) {
	panic(ErrNotImplementedYet)
}

func (s *ByteSlice) appendSequence(ctx *Context, seq Sequence) {
	panic(ErrNotImplementedYet)
}

// Byte implements Value.
type Byte byte

func (b Byte) Int64() (n int64, signed bool) {
	return int64(b), false
}

// BytesConcatenation is a lazy concatenation of values that can form a byte slice, BytesConcatenation implements BytesLike.
type BytesConcatenation struct {
	NoReprMixin
	elements   []BytesLike
	totalLen   int
	finalBytes []byte // empty by default
}

func (c *BytesConcatenation) GetOrBuildBytes() *ByteSlice {
	if c.Len() > 0 && len(c.finalBytes) == 0 {
		slice := make([]byte, c.totalLen)
		pos := 0
		for _, elem := range c.elements {
			copy(slice[pos:pos+elem.Len()], elem.GetOrBuildBytes().Bytes)
			pos += elem.Len()
		}
		c.finalBytes = slice
		//get rid of elements to allow garbage collection ?
	}
	return &ByteSlice{
		Bytes:         c.finalBytes,
		IsDataMutable: false,
	}
}

func (c *BytesConcatenation) Mutable() bool {
	return c.elements[0].Mutable()
}

func (c *BytesConcatenation) set(ctx *Context, i int, v Value) {
	if !c.Mutable() {
		panic(fmt.Errorf("attempt to write a readonly bytes concatenation"))
	}
	panic(fmt.Errorf("cannot mutate bytes concatenation %w", ErrNotImplementedYet))
}

func (c *BytesConcatenation) setSlice(ctx *Context, start, end int, v Value) {
	if !c.Mutable() {
		panic(fmt.Errorf("attempt to write a readonly bytes concatenation"))
	}
	panic(fmt.Errorf("cannot mutate bytes concatenation %w", ErrNotImplementedYet))
}

func (c *BytesConcatenation) Len() int {
	return c.totalLen
}

func (c *BytesConcatenation) At(ctx *Context, i int) Value {
	elementIndex := 0
	pos := 0
	for pos < i {
		element := c.elements[elementIndex]
		if pos+element.Len() >= i {
			return element.At(ctx, i-pos)
		}
		elementIndex++
		pos += element.Len()
	}

	panic(ErrIndexOutOfRange)
}

func (c *BytesConcatenation) slice(start, end int) Sequence {
	//TODO: change implementation + make the function stoppable for large slices

	return c.GetOrBuildBytes().slice(start, end)
}
