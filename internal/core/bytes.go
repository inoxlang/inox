package core

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"sync"

	"github.com/inoxlang/inox/internal/mimeconsts"
	utils "github.com/inoxlang/inox/internal/utils/common"
)

var (
	ErrAttemptToMutateReadonlyByteSlice            = errors.New("attempt to write a readonly byte slice")
	ErrAttemptToCreateMutableSpecificTypeByteSlice = errors.New("attempt to create a mutable byte slice with specific content type")

	_ = []GoBytes{(*ByteSlice)(nil)}
	_ = []BytesLike{(*ByteSlice)(nil), (*BytesConcatenation)(nil)}
)

// The GoString interfaces is implemented by types that are of kind reflect.String
type GoBytes interface {
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

// ByteSlice implements Value, its mutability is set at creation.
type ByteSlice struct {
	bytes         []byte
	isDataMutable bool
	contentType   Mimetype
	constraintId  ConstraintId

	// exclusive access for initializing .watchers & .mutationCallbacks only.
	lock              sync.Mutex
	watchers          *ValueWatchers
	mutationCallbacks *MutationCallbacks
}

func NewByteSlice(bytes []byte, mutable bool, contentType Mimetype) *ByteSlice {
	//TODO: check content type
	if contentType != "" && mutable {
		panic(ErrAttemptToCreateMutableSpecificTypeByteSlice)
	}
	return &ByteSlice{bytes: bytes, isDataMutable: mutable, contentType: contentType}
}

func NewMutableByteSlice(bytes []byte, contentType Mimetype) *ByteSlice {
	return NewByteSlice(bytes, true, contentType)
}

func NewImmutableByteSlice(bytes []byte, contentType Mimetype) *ByteSlice {
	return NewByteSlice(bytes, false, contentType)
}

func (slice *ByteSlice) UnderlyingBytes() []byte {
	return slice.bytes
}

func (slice *ByteSlice) UnsafeBytesAsString() string {
	return utils.BytesAsString(slice.UnderlyingBytes())
}

func (slice *ByteSlice) GetOrBuildBytes() *ByteSlice {
	return slice
}

func (slice *ByteSlice) Mutable() bool {
	return slice.isDataMutable
}

// ContentType returns the content type specified at creation.
// If no content type was specified mimeconsts.APP_OCTET_STREAM_CTYPE is returned instead.
func (slice *ByteSlice) ContentType() Mimetype {
	if slice.contentType == "" {
		return mimeconsts.APP_OCTET_STREAM_CTYPE
	}
	return slice.contentType
}

func (slice *ByteSlice) slice(start, end int) Sequence {
	sliceCopy := make([]byte, end-start)
	copy(sliceCopy, slice.bytes[start:end])

	return &ByteSlice{bytes: sliceCopy, isDataMutable: slice.isDataMutable}
}

func (slice *ByteSlice) Len() int {
	return len(slice.bytes)
}

func (slice *ByteSlice) At(ctx *Context, i int) Value {
	return Byte(slice.bytes[i])
}

func (slice *ByteSlice) set(ctx *Context, i int, v Value) {
	if !slice.isDataMutable {
		panic(ErrAttemptToMutateReadonlyByteSlice)
	}
	slice.bytes[i] = byte(v.(Byte))

	mutation := NewSetElemAtIndexMutation(ctx, i, v.(Byte), ShallowWatching, Path("/"+strconv.Itoa(i)))

	slice.mutationCallbacks.CallMicrotasks(ctx, mutation)
	slice.watchers.InformAboutAsync(ctx, mutation, ShallowWatching, true)
}

func (slice *ByteSlice) SetSlice(ctx *Context, start, end int, seq Sequence) {
	if !slice.isDataMutable {
		panic(ErrAttemptToMutateReadonlyByteSlice)
	}

	if seq.Len() != end-start {
		panic(errors.New(FormatIndexableShouldHaveLen(end - start)))
	}

	for i := start; i < end; i++ {
		slice.bytes[i] = byte(seq.At(ctx, i-start).(Byte))
	}

	path := Path("/" + strconv.Itoa(int(start)) + ".." + strconv.Itoa(int(end-1)))
	mutation := NewSetSliceAtRangeMutation(ctx, NewIntRange(int64(start), int64(end-1)), seq.(Serializable), ShallowWatching, path)

	slice.mutationCallbacks.CallMicrotasks(ctx, mutation)
	slice.watchers.InformAboutAsync(ctx, mutation, ShallowWatching, true)
}

func (s *ByteSlice) insertElement(ctx *Context, v Value, i Int) {
	if !s.isDataMutable {
		panic(ErrAttemptToMutateReadonlyByteSlice)
	}

	b := v.(Byte)
	s.bytes = append(s.bytes, 0)
	copy(s.bytes[i+1:], s.bytes[i:len(s.bytes)-1])
	s.bytes[i] = byte(b)

	mutation := NewInsertElemAtIndexMutation(ctx, int(i), b, ShallowWatching, Path("/"+strconv.Itoa(int(i))))

	s.mutationCallbacks.CallMicrotasks(ctx, mutation)
	s.watchers.InformAboutAsync(ctx, mutation, ShallowWatching, true)
}

func (s *ByteSlice) removePosition(ctx *Context, i Int) {
	if !s.isDataMutable {
		panic(ErrAttemptToMutateReadonlyByteSlice)
	}

	if int(i) > len(s.bytes) || i < 0 {
		panic(ErrIndexOutOfRange)
	}

	if int(i) == len(s.bytes)-1 { // remove last position
		s.bytes = s.bytes[:len(s.bytes)-1]
	} else {
		copy(s.bytes[i:], s.bytes[i+1:])
		s.bytes = s.bytes[:len(s.bytes)-1]
	}

	mutation := NewRemovePositionMutation(ctx, int(i), ShallowWatching, Path("/"+strconv.Itoa(int(i))))

	s.mutationCallbacks.CallMicrotasks(ctx, mutation)
	s.watchers.InformAboutAsync(ctx, mutation, ShallowWatching, true)
}

func (s *ByteSlice) removePositionRange(ctx *Context, r IntRange) {
	if !s.isDataMutable {
		panic(ErrAttemptToMutateReadonlyByteSlice)
	}

	start := int(r.KnownStart())
	end := int(r.InclusiveEnd())

	if start > len(s.bytes) || start < 0 || end >= len(s.bytes) || end < 0 {
		panic(ErrIndexOutOfRange)
	}

	if end == len(s.bytes)-1 { // remove trailing sub slice
		s.bytes = s.bytes[:len(s.bytes)-r.Len()]
	} else {
		copy(s.bytes[start:], s.bytes[end+1:])
		s.bytes = s.bytes[:len(s.bytes)-r.Len()]
	}

	path := Path("/" + strconv.Itoa(int(r.KnownStart())) + ".." + strconv.Itoa(int(r.InclusiveEnd())))
	mutation := NewRemovePositionRangeMutation(ctx, r, ShallowWatching, path)

	s.mutationCallbacks.CallMicrotasks(ctx, mutation)
	s.watchers.InformAboutAsync(ctx, mutation, ShallowWatching, true)
}

func (s *ByteSlice) insertSequence(ctx *Context, seq Sequence, i Int) {
	if !s.isDataMutable {
		panic(ErrAttemptToMutateReadonlyByteSlice)
	}

	// TODO: lock sequence
	seqLen := seq.Len()
	if seqLen == 0 {
		return
	}

	if cap(s.bytes)-len(s.bytes) < seqLen {
		newSlice := make([]byte, len(s.bytes)+seqLen)
		copy(newSlice, s.bytes)
		s.bytes = newSlice
	} else {
		s.bytes = s.bytes[:len(s.bytes)+seqLen]
	}

	copy(s.bytes[int(i)+seqLen:], s.bytes[i:])
	for ind := 0; ind < seqLen; ind++ {
		s.bytes[int(i)+ind] = byte(seq.At(ctx, ind).(Byte))
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

func (b Byte) Int64() int64 {
	return int64(b)
}

func (b Byte) IsSigned() bool {
	return false
}

// BytesConcatenation is a lazy concatenation of values that can form a byte slice, BytesConcatenation implements BytesLike.
type BytesConcatenation struct {
	elements   []BytesLike
	totalLen   int
	finalBytes []byte // empty by default
}

func ConcatBytesLikes(bytesLikes ...BytesLike) (BytesLike, error) {
	//TODO: concatenate small sequences together

	totalLen := 0

	for i, bytesLike := range bytesLikes {
		if bytesLike.Mutable() {
			b := slices.Clone(bytesLike.GetOrBuildBytes().bytes) // TODO: use Copy On Write
			bytesLikes[i] = NewByteSlice(b, false, "")
		}
		totalLen += bytesLike.Len()
	}

	if len(bytesLikes) == 1 {
		return bytesLikes[0], nil
	}

	return &BytesConcatenation{
		elements: slices.Clone(bytesLikes),
		totalLen: totalLen,
	}, nil
}

func NewBytesConcatenation(bytesLikes ...BytesLike) *BytesConcatenation {
	if len(bytesLikes) < 2 {
		panic(errors.New("not enough elements"))
	}

	var totalLen int

	for i, bytesLike := range bytesLikes {
		if bytesLike.Mutable() {
			b := slices.Clone(bytesLike.GetOrBuildBytes().bytes) // TODO: use Copy On Write
			bytesLikes[i] = NewByteSlice(b, false, "")
		}
		totalLen += bytesLike.Len()
	}

	return &BytesConcatenation{
		elements: slices.Clone(bytesLikes),
		totalLen: totalLen,
	}
}

func (c *BytesConcatenation) GetOrBuildBytes() *ByteSlice {
	if c.Len() > 0 && len(c.finalBytes) == 0 {
		slice := make([]byte, c.totalLen)
		pos := 0
		for _, elem := range c.elements {
			copy(slice[pos:pos+elem.Len()], elem.GetOrBuildBytes().bytes)
			pos += elem.Len()
		}
		c.finalBytes = slice
		//get rid of elements to allow garbage collection ?
	}
	return &ByteSlice{
		bytes:         c.finalBytes,
		isDataMutable: false,
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
