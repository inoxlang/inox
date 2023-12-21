package core

import (
	"errors"
	"fmt"
	"io"
	"sync/atomic"
	"time"

	"github.com/inoxlang/inox/internal/utils"
)

var (
	ErrEndOfStream            = errors.New("end of stream")
	ErrStreamElemWaitTimeout  = errors.New("stream element wait timeout")
	ErrStreamChunkWaitTimeout = errors.New("stream chunk wait timeout")
	ErrTempDriedUpSource      = errors.New("temporarily dried up source")
	ErrDefDriedUpSource       = errors.New("definitively dried up source")

	_ = []ReadableStream{(*wrappedWatcherStream)(nil), (*ElementsStream)(nil), (*ReadableByteStream)(nil), (*ConfluenceStream)(nil)}

	WRAPPED_WATCHER_STREAM_CHUNK_DATA_TYPE = &ListPattern{generalElementPattern: ANYVAL_PATTERN}
	ELEMENTS_STREAM_CHUNK_DATA_TYPE        = &ListPattern{generalElementPattern: ANYVAL_PATTERN}
	BYTESTREAM_CHUNK_DATA_TYPE             = BYTESLICE_PATTERN
)

const (
	BYTE_STREAM_BUFF_GROWTH_FACTOR          = 2
	BYTE_STREAM_MINIMUM_MICRO_WAIT_DURATION = 100 * time.Microsecond

	NOT_STARTED_CONFLUENCE_STREAM = 0
	STARTED_CONFLUENCE_STREAM     = 1
	STOPPED_CONFLUENCE_STREAM     = 2
)

type StreamSource interface {
	Value

	Stream(ctx *Context, optionalConfig *ReadableStreamConfiguration) ReadableStream
}

type ReadableStreamConfiguration struct {
	Filter Pattern
}

type ReadableStream interface {
	StreamSource

	// WaitNext should be called by a single goroutine, filter can be nil.
	// WaitNext should return the EndOfStream error only after the last element has been returned
	WaitNext(ctx *Context, filter Pattern, timeout time.Duration) (Value, error)

	// WaitNextChunk should be called by a single goroutine, filter can be nil.
	// If there is a non-nil error that is not EndOfStream the chunk should be nil
	WaitNextChunk(ctx *Context, filter Pattern, sizeRange IntRange, timeout time.Duration) (*DataChunk, error)

	Stop()

	IsStopped() bool

	IsMainlyChunked() bool

	ChunkDataType() Pattern
}

// stream implementations

// all watchers should return this stream.
type wrappedWatcherStream struct {
	watcher Watcher
}

func (s *wrappedWatcherStream) WaitNext(ctx *Context, filter Pattern, timeout time.Duration) (Value, error) {
	//TODO: add configuration's filter ?
	next, err := s.watcher.WaitNext(ctx, filter, timeout)
	if errors.Is(err, ErrStoppedWatcher) {
		return nil, ErrEndOfStream
	}
	if errors.Is(err, ErrWatchTimeout) {
		return nil, ErrStreamElemWaitTimeout
	}
	return next, err
}

func (s *wrappedWatcherStream) WaitNextChunk(ctx *Context, filter Pattern, sizeRange IntRange, timeout time.Duration) (*DataChunk, error) {
	min := sizeRange.KnownStart()

	chunkData := make([]Serializable, int(min))
	startTime := time.Now()

	//TODO: return chunk even if length < min ?

	newChunk := func(ind int) *DataChunk {
		return &DataChunk{
			data: NewWrappedValueList(chunkData[:ind]...),
			merge: func(c, other *DataChunk) error {
				otherList, ok := other.data.(*List)
				if !ok {
					return fmt.Errorf("cannot merge a chunk of tuple with a chunk with data of type %T", other.data)
				}

				c.data.(*List).append(ctx, otherList.GetOrBuildElements(ctx)...)
				return nil
			},
		}
	}

	for i := 0; i < int(min); i++ {
		next, err := s.WaitNext(ctx, filter, timeout)
		if errors.Is(err, ErrStoppedWatcher) {
			return newChunk(i), ErrEndOfStream
		}

		if errors.Is(err, ErrWatchTimeout) || time.Since(startTime) >= timeout {
			return newChunk(i), ErrStreamChunkWaitTimeout
		}

		if err != nil {
			return nil, err
		}

		chunkData[i] = next.(Serializable)
	}

	return newChunk(len(chunkData)), nil
}

func (s *wrappedWatcherStream) Stop() {
	s.watcher.Stop()
}

func (s *wrappedWatcherStream) IsStopped() bool {
	return s.watcher.IsStopped()
}

func (s *wrappedWatcherStream) IsMainlyChunked() bool {
	return false
}

func (s *wrappedWatcherStream) ChunkDataType() Pattern {
	return WRAPPED_WATCHER_STREAM_CHUNK_DATA_TYPE
}

func (s *wrappedWatcherStream) Stream(ctx *Context, config *ReadableStreamConfiguration) ReadableStream {
	return s
}

func (w stoppedWatcher) Stream(ctx *Context, config *ReadableStreamConfiguration) ReadableStream {
	return &wrappedWatcherStream{watcher: w}
}

func (w *GenericWatcher) Stream(ctx *Context, config *ReadableStreamConfiguration) ReadableStream {
	return &wrappedWatcherStream{watcher: w}
}

func (w *PeriodicWatcher) Stream(ctx *Context, config *ReadableStreamConfiguration) ReadableStream {
	return &wrappedWatcherStream{watcher: w}
}

func (w *joinedWatchers) Stream(ctx *Context, config *ReadableStreamConfiguration) ReadableStream {
	return &wrappedWatcherStream{watcher: w}
}

// An ElementsStream represents a stream of known elements, ElementsStream implements Value.
type ElementsStream struct {
	filter    Pattern
	nextIndex int
	stopped   atomic.Bool
	elements  []Value
}

func ToReadableStream(ctx *Context, v Value, optionalFilter Pattern) ReadableStream {
	if optionalFilter == nil {
		panic(errors.New("stream filter not supported yet"))
		//optionalFilter = ANYVAL_PATTERN
	}

	switch val := v.(type) {
	case Readable:
		reader := val.Reader()
		return NewByteStream(func(s *ReadableByteStream, p []byte) (int, error) {
			n, err := reader.Read(p)
			if errors.Is(err, io.EOF) {
				err = ErrDefDriedUpSource
			}
			return n, err
		}, func(s *ReadableByteStream) (byte, error) {
			var b [1]byte
			n, err := reader.Read(b[:])

			if errors.Is(err, io.EOF) {
				if n == 0 {
					err = ErrDefDriedUpSource
				} else {
					err = nil
				}
			}

			if err != nil {
				return 0, err
			}
			return b[0], err
		}, nil, nil)

	case Indexable:
		len := val.Len()
		elements := make([]Value, len)

		for i := 0; i < len; i++ {
			elements[i] = val.At(ctx, i)
		}
		return NewElementsStream(elements, optionalFilter)
	case Iterable:
		panic(errors.New("failed to create stream from iterable: not implemented yet"))
	default:
		panic(fmt.Errorf("failed to create stream from provided value: %T", v))
	}
}

func NewElementsStream(elements []Value, filter Pattern) *ElementsStream {
	return &ElementsStream{elements: elements, filter: filter}
}

func (s *ElementsStream) Stream(ctx *Context, config *ReadableStreamConfiguration) ReadableStream {
	return s
}

func (s *ElementsStream) WaitNext(ctx *Context, filter Pattern, timeout time.Duration) (Value, error) {
	if s.IsStopped() {
		return nil, ErrEndOfStream
	}

	for j := s.nextIndex; j < len(s.elements); j++ {
		elem := s.elements[j]
		s.nextIndex++
		if (s.filter == nil || s.filter.Test(ctx, elem)) && (filter == nil || filter.Test(ctx, elem)) {
			if s.nextIndex >= len(s.elements) {
				s.Stop()
			}
			return elem, nil
		}
	}

	return nil, ErrEndOfStream
}

func (s *ElementsStream) WaitNextChunk(ctx *Context, filter Pattern, sizeRange IntRange, timeout time.Duration) (*DataChunk, error) {
	if s.IsStopped() {
		return nil, ErrEndOfStream
	}

	chunkData := make([]Serializable, sizeRange.KnownStart())
	i := 0

	for j := s.nextIndex; j < len(s.elements) && i < len(chunkData); j++ {
		elem := s.elements[j]
		s.nextIndex++
		if (s.filter == nil || s.filter.Test(ctx, elem)) && (filter == nil || filter.Test(ctx, elem)) {
			chunkData[i] = elem.(Serializable)
			i++
		}
	}

	var err error

	if s.nextIndex >= len(s.elements) {
		s.Stop()
		err = ErrEndOfStream
	}

	return &DataChunk{
		data: NewWrappedValueList(chunkData[:i]...),
	}, err
}

func (s *ElementsStream) Stop() {
	s.stopped.Store(true)
}

func (s *ElementsStream) IsStopped() bool {
	return s.stopped.Load()
}

func (s *ElementsStream) IsMainlyChunked() bool {
	return false
}

func (s *ElementsStream) ChunkDataType() Pattern {
	return ELEMENTS_STREAM_CHUNK_DATA_TYPE
}

// A ReadableByteStream represents a stream of bytes, ElementsStream implements Value.
type ReadableByteStream struct {
	filter  Pattern
	stopped atomic.Bool

	//readSourceBytes should return ErrDefDriedUpSource when the source will no longer give bytes (even if some bytes are read)
	readSourceBytes func(s *ReadableByteStream, p []byte) (int, error)

	//readSourceByte should not return an error if a byte is returned AND the source is dried up
	readSourceByte func(s *ReadableByteStream) (byte, error)

	rechargedSourceSignal chan struct{}
}

func NewByteStream(
	readSourceBytes func(s *ReadableByteStream, p []byte) (int, error),
	readSourceByte func(s *ReadableByteStream) (byte, error),
	rechargedSourceSignal chan struct{},
	filter Pattern,
) *ReadableByteStream {
	return &ReadableByteStream{
		readSourceBytes:       readSourceBytes,
		readSourceByte:        readSourceByte,
		filter:                filter,
		rechargedSourceSignal: rechargedSourceSignal,
	}
}

func (s *ReadableByteStream) Stream(ctx *Context, config *ReadableStreamConfiguration) ReadableStream {
	return s
}

func (s *ReadableByteStream) WaitNext(ctx *Context, filter Pattern, timeout time.Duration) (Value, error) {
	if s.IsStopped() {
		return nil, ErrEndOfStream
	}

	start := time.Now()
	deadline := start.Add(timeout)
	var timeoutTimer *time.Timer
	defer func() {
		if timeoutTimer != nil {
			timeoutTimer.Stop()
		}
	}()

	for time.Since(start) < timeout {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		var streamErr error
		b, err := s.readSourceByte(s)

		if err == nil {
			return Byte(b), nil
		}

		if errors.Is(err, ErrTempDriedUpSource) {
			// wait
			if s.rechargedSourceSignal != nil {
				remainingTime := time.Until(deadline)
				if timeoutTimer == nil {
					timeoutTimer = time.NewTimer(remainingTime)
				}

				select {
				case <-timeoutTimer.C:
					return nil, ErrStreamElemWaitTimeout
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-s.rechargedSourceSignal:
					break
				}
			} else {
				ctx.Sleep(BYTE_STREAM_MINIMUM_MICRO_WAIT_DURATION)
			}
			continue
		}

		if errors.Is(err, ErrDefDriedUpSource) {
			streamErr = ErrEndOfStream
		} else if err != nil && !errors.Is(err, ErrTempDriedUpSource) {
			streamErr = err
		}

		return nil, streamErr
	}

	return nil, ErrStreamElemWaitTimeout
}

func (s *ReadableByteStream) WaitNextChunk(ctx *Context, filter Pattern, sizeRange IntRange, timeout time.Duration) (*DataChunk, error) {
	if s.IsStopped() {
		return nil, ErrEndOfStream
	}

	start := time.Now()
	deadline := start.Add(timeout)
	var timeoutTimer *time.Timer
	defer func() {
		if timeoutTimer != nil {
			timeoutTimer.Stop()
		}
	}()

	min := int(sizeRange.KnownStart())
	max := int(sizeRange.InclusiveEnd())

	chunkData := make([]byte, min)
	i := 0
	read := 0

	var streamErr error

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		if time.Since(start) > timeout {
			if i == 0 { // if there is no time left & the buffer is stil empty we return an error
				streamErr = ErrStreamChunkWaitTimeout
			}
			break
		}

		n, err := s.readSourceBytes(s, chunkData[i:])
		read += n
		i = read

		//if the buffer is full and the source is not empty we grow the slice & we continue reading
		if err == nil && read >= len(chunkData) && len(chunkData) < max {
			prevData := chunkData
			chunkData = make([]byte, utils.Min(max, BYTE_STREAM_BUFF_GROWTH_FACTOR*len(prevData)))
			copy(chunkData, prevData)
			continue
		}

		// even if there nothing in the source we keep reading
		if errors.Is(err, ErrTempDriedUpSource) && read < min {
			//wait
			if s.rechargedSourceSignal != nil {
				remainingTime := time.Until(deadline)
				if timeoutTimer == nil {
					timeoutTimer = time.NewTimer(remainingTime)
				}

				select {
				case <-timeoutTimer.C:
					return nil, ErrStreamChunkWaitTimeout
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-s.rechargedSourceSignal:
				}
			} else {
				ctx.Sleep(BYTE_STREAM_MINIMUM_MICRO_WAIT_DURATION)
			}

			continue
		}

		// handle error
		if errors.Is(err, ErrDefDriedUpSource) {
			streamErr = ErrEndOfStream
		} else if err != nil && !errors.Is(err, ErrTempDriedUpSource) {
			streamErr = err
		}

		break
	}

	if streamErr == nil || streamErr == ErrEndOfStream {
		return &DataChunk{
			data: NewByteSlice(chunkData[:i], true, ""),
			merge: func(c, other *DataChunk) error {
				otherBytes, ok := other.data.(*ByteSlice)
				if !ok {
					return fmt.Errorf("cannot merge a chunk of bytes with a chunk of %T", other.data)
				}
				bytes := c.data.(*ByteSlice)
				if !bytes.IsMutable() {
					return errors.New("chunk is not mutable")
				}
				bytes.bytes = append(bytes.bytes, otherBytes.bytes...)

				return nil
			},
		}, streamErr
	}

	return nil, streamErr
}

func (s *ReadableByteStream) Stop() {
	//TODO: stop source
	s.stopped.Store(true)
}

func (s *ReadableByteStream) IsStopped() bool {
	return s.stopped.Load()
}

func (s *ReadableByteStream) IsMainlyChunked() bool {
	return true
}

func (s *ReadableByteStream) ChunkDataType() Pattern {
	return BYTESTREAM_CHUNK_DATA_TYPE
}

func (r *RingBuffer) Stream(ctx *Context, config *ReadableStreamConfiguration) ReadableStream {
	//TODO: support filter

	r.lock.Lock()
	defer r.lock.Unlock()

	if r.availableDataSignal == nil {
		r.availableDataSignal = make(chan struct{}, 1)
	}

	return NewByteStream(
		func(s *ReadableByteStream, p []byte) (int, error) {
			//TODO: return EoS when buffer is closed
			n, err := r.Read(p)

			if err == ErrEmptyRingBuffer {
				err = ErrTempDriedUpSource
			}

			return n, err
		},
		func(s *ReadableByteStream) (byte, error) {
			b, err := r.ReadByte()
			if err == ErrEmptyRingBuffer {
				return 0, ErrTempDriedUpSource
			}
			return b, nil
		},
		r.availableDataSignal,
		nil,
	)
}

// A ConfluenceStream is a ReadableStream that results from the merger of 2 or more streams.
// ConfluenceStream was developped to combine the output & error output streams of the inox REPL but the current implementation is somewhat incorrect.
// TODO: change the way the data is read, one possibility is to make the streams PUSH their data in a buffer.
type ConfluenceStream struct {
	status atomic.Int32 //0: not started, 1: started, 2: stopped

	streams       []ReadableStream
	mainlyChunked bool
	chunkDataType Pattern
}

func NewConfluenceStream(ctx *Context, streams []ReadableStream) (*ConfluenceStream, error) {
	if len(streams) < 2 {
		return nil, errors.New("at least 2 streams are required to create a confluence stream")
	}

	chunked := streams[0].IsMainlyChunked()
	chunkDataType := streams[0].ChunkDataType()
	for _, s := range streams[1:] {
		if s.IsMainlyChunked() != chunked {
			return nil, errors.New("a confluence streams can only merge streams that have the same 'main chunkness' (all mainly chunked OR all NOT mainly chunked)")
		}
		if !chunkDataType.Equal(ctx, s.ChunkDataType(), map[uintptr]uintptr{}, 0) {
			return nil, errors.New("a confluence streams can only merge streams with the same type of data")
		}
	}
	//TODO: check that the streams have the same kind of elements/chunks
	return &ConfluenceStream{streams: streams, mainlyChunked: chunked, chunkDataType: chunkDataType}, nil
}

func (s *ConfluenceStream) Stream(ctx *Context, config *ReadableStreamConfiguration) ReadableStream {
	return s
}

func (s *ConfluenceStream) WaitNext(ctx *Context, filter Pattern, timeout time.Duration) (Value, error) {
	status := s.status.Load()

	if status == STOPPED_CONFLUENCE_STREAM {
		return nil, ErrEndOfStream
	}

	singleStreamTimeout := timeout / time.Duration(len(s.streams))
	streamEnd := true

	for i, stream := range s.streams {
		if stream == nil {
			continue
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		next, err := stream.WaitNext(ctx, filter, singleStreamTimeout)

		if errors.Is(err, ErrEndOfStream) {
			s.streams[i] = nil
			continue
		}

		streamEnd = false

		if errors.Is(err, ErrStreamElemWaitTimeout) {
			continue
		}

		if err != nil {
			return nil, err
		}

		return next, nil
	}

	if streamEnd {
		return nil, ErrEndOfStream
	}

	return nil, ErrStreamElemWaitTimeout
}

func (s *ConfluenceStream) WaitNextChunk(ctx *Context, filter Pattern, sizeRange IntRange, timeout time.Duration) (*DataChunk, error) {
	status := s.status.Load()

	if status == STOPPED_CONFLUENCE_STREAM {
		return nil, ErrEndOfStream
	}

	singleStreamTimeout := timeout / time.Duration(len(s.streams))
	streamEnd := true

	var finalChunk *DataChunk

	for i, stream := range s.streams {
		if stream == nil {
			continue
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		chunk, err := stream.WaitNextChunk(ctx, filter, sizeRange, singleStreamTimeout)

		if errors.Is(err, ErrEndOfStream) {
			s.streams[i] = nil
		} else {
			streamEnd = false
		}

		if errors.Is(err, ErrStreamChunkWaitTimeout) || (chunk != nil && chunk.ElemCount() == 0) {
			continue
		}

		if err != nil {
			return nil, err
		}

		if finalChunk == nil {
			finalChunk = chunk
		} else {
			err := finalChunk.MergeWith(ctx, chunk)
			if err != nil {
				return nil, err
			}
		}

		if finalChunk.ElemCount() >= int(sizeRange.KnownStart()) {
			return finalChunk, nil
		}

		//TODO: if chunk is too big truncate it & keep additional data for later
	}

	if streamEnd {
		return finalChunk, ErrEndOfStream
	}

	if finalChunk == nil || finalChunk.ElemCount() == 0 {
		return nil, ErrStreamChunkWaitTimeout
	}

	return finalChunk, nil
}

func (s *ConfluenceStream) Stop() {
	//TODO: stop sink ?
	s.status.Store(2)
}

func (s *ConfluenceStream) IsStopped() bool {
	return s.status.Load() == STOPPED_CONFLUENCE_STREAM
}

func (s *ConfluenceStream) IsMainlyChunked() bool {
	return s.mainlyChunked
}

func (s *ConfluenceStream) ChunkDataType() Pattern {
	return BYTESTREAM_CHUNK_DATA_TYPE
}
