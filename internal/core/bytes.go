package core

import (
	"errors"
	"fmt"
	"strconv"
	"sync"

	"github.com/inoxlang/inox/internal/mimeconsts"
)

var (
	ErrAttemptToMutateReadonlyByteSlice            = errors.New("attempt to write a readonly byte slice")
	ErrAttemptToCreateMutableSpecificTypeByteSlice = errors.New("attempt to create a mutable byte slice with specific content type")
	_                                              = []WrappedBytes{&ByteSlice{}}
	_                                              = []BytesLike{&ByteSlice{}, &BytesConcatenation{}}
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
	Iterable
	GetOrBuildBytes() *ByteSlice
	Mutable() bool
}

// ByteSlice implements Value.
type ByteSlice struct {
	Bytes         []byte
	IsDataMutable bool
	contentType   Mimetype
	constraintId  ConstraintId

	lock              sync.Mutex // exclusive access for initializing .watchers & .mutationCallbacks
	watchers          *ValueWatchers
	mutationCallbacks *MutationCallbacks
}

func NewByteSlice(bytes []byte, mutable bool, contentType Mimetype) *ByteSlice {
	//TODO: check content type
	if contentType != "" && mutable {
		panic(ErrAttemptToCreateMutableSpecificTypeByteSlice)
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
		return mimeconsts.APP_OCTET_STREAM_CTYPE
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
		panic(ErrAttemptToMutateReadonlyByteSlice)
	}
	slice.Bytes[i] = byte(v.(Byte))

	mutation := NewSetElemAtIndexMutation(ctx, i, v.(Byte), ShallowWatching, Path("/"+strconv.Itoa(i)))

	slice.mutationCallbacks.CallMicrotasks(ctx, mutation)
	slice.watchers.InformAboutAsync(ctx, mutation, ShallowWatching, true)
}

func (slice *ByteSlice) SetSlice(ctx *Context, start, end int, seq Sequence) {
	if !slice.IsDataMutable {
		panic(ErrAttemptToMutateReadonlyByteSlice)
	}

	if seq.Len() != end-start {
		panic(errors.New(FormatIndexableShouldHaveLen(end - start)))
	}

	for i := start; i < end; i++ {
		slice.Bytes[i] = byte(seq.At(ctx, i-start).(Byte))
	}

	path := Path("/" + strconv.Itoa(int(start)) + ".." + strconv.Itoa(int(end-1)))
	mutation := NewSetSliceAtRangeMutation(ctx, NewIncludedEndIntRange(int64(start), int64(end-1)), seq.(Serializable), ShallowWatching, path)

	slice.mutationCallbacks.CallMicrotasks(ctx, mutation)
	slice.watchers.InformAboutAsync(ctx, mutation, ShallowWatching, true)
}

func (s *ByteSlice) insertElement(ctx *Context, v Value, i Int) {
	if !s.IsDataMutable {
		panic(ErrAttemptToMutateReadonlyByteSlice)
	}

	b := v.(Byte)
	s.Bytes = append(s.Bytes, 0)
	copy(s.Bytes[i+1:], s.Bytes[i:len(s.Bytes)-1])
	s.Bytes[i] = byte(b)

	mutation := NewInsertElemAtIndexMutation(ctx, int(i), b, ShallowWatching, Path("/"+strconv.Itoa(int(i))))

	s.mutationCallbacks.CallMicrotasks(ctx, mutation)
	s.watchers.InformAboutAsync(ctx, mutation, ShallowWatching, true)
}

func (s *ByteSlice) removePosition(ctx *Context, i Int) {
	if !s.IsDataMutable {
		panic(ErrAttemptToMutateReadonlyByteSlice)
	}

	if int(i) > len(s.Bytes) || i < 0 {
		panic(ErrIndexOutOfRange)
	}

	if int(i) == len(s.Bytes)-1 { // remove last position
		s.Bytes = s.Bytes[:len(s.Bytes)-1]
	} else {
		copy(s.Bytes[i:], s.Bytes[i+1:])
		s.Bytes = s.Bytes[:len(s.Bytes)-1]
	}

	mutation := NewRemovePositionMutation(ctx, int(i), ShallowWatching, Path("/"+strconv.Itoa(int(i))))

	s.mutationCallbacks.CallMicrotasks(ctx, mutation)
	s.watchers.InformAboutAsync(ctx, mutation, ShallowWatching, true)
}

func (s *ByteSlice) removePositionRange(ctx *Context, r IntRange) {
	if !s.IsDataMutable {
		panic(ErrAttemptToMutateReadonlyByteSlice)
	}

	start := int(r.KnownStart())
	end := int(r.InclusiveEnd())

	if start > len(s.Bytes) || start < 0 || end >= len(s.Bytes) || end < 0 {
		panic(ErrIndexOutOfRange)
	}

	if end == len(s.Bytes)-1 { // remove trailing sub slice
		s.Bytes = s.Bytes[:len(s.Bytes)-r.Len()]
	} else {
		copy(s.Bytes[start:], s.Bytes[end+1:])
		s.Bytes = s.Bytes[:len(s.Bytes)-r.Len()]
	}

	path := Path("/" + strconv.Itoa(int(r.KnownStart())) + ".." + strconv.Itoa(int(r.InclusiveEnd())))
	mutation := NewRemovePositionRangeMutation(ctx, r, ShallowWatching, path)

	s.mutationCallbacks.CallMicrotasks(ctx, mutation)
	s.watchers.InformAboutAsync(ctx, mutation, ShallowWatching, true)
}

func (s *ByteSlice) insertSequence(ctx *Context, seq Sequence, i Int) {
	if !s.IsDataMutable {
		panic(ErrAttemptToMutateReadonlyByteSlice)
	}

	// TODO: lock sequence
	seqLen := seq.Len()
	if seqLen == 0 {
		return
	}

	if cap(s.Bytes)-len(s.Bytes) < seqLen {
		newSlice := make([]byte, len(s.Bytes)+seqLen)
		copy(newSlice, s.Bytes)
		s.Bytes = newSlice
	} else {
		s.Bytes = s.Bytes[:len(s.Bytes)+seqLen]
	}

	copy(s.Bytes[int(i)+seqLen:], s.Bytes[i:])
	for ind := 0; ind < seqLen; ind++ {
		s.Bytes[int(i)+ind] = byte(seq.At(ctx, ind).(Byte))
	}

	path := Path("/" + strconv.Itoa(int(i)))
	mutation := NewInsertSequenceAtIndexMutation(ctx, int(i), seq, ShallowWatching, path)

	s.mutationCallbacks.CallMicrotasks(ctx, mutation)
	s.watchers.InformAboutAsync(ctx, mutation, ShallowWatching, true)
}

func (s *ByteSlice) appendSequence(ctx *Context, seq Sequence) {
	s.insertSequence(ctx, seq, Int(s.Len()))
}

// Byte implements Value.
type Byte byte

func (b Byte) Int64() (n int64, signed bool) {
	return int64(b), false
}

// BytesConcatenation is a lazy concatenation of values that can form a byte slice, BytesConcatenation implements BytesLike.
type BytesConcatenation struct {
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

func (c *BytesConcatenation) SetSlice(ctx *Context, start, end int, seq Sequence) {
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
