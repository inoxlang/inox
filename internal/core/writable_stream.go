package core

import (
	"errors"
	"fmt"
	"sync/atomic"
)

var (
	ErrStoppedWritableStream  = errors.New("writable stream is stopped")
	ErrInvalidStreamElement   = errors.New("invalid stream element")
	ErrInvalidStreamChunkData = errors.New("invalid stream chunk data")

	_ = []StreamSink{&RingBuffer{}}
	_ = []WritableStream{&WritableByteStream{}}
)

type StreamSink interface {
	Value

	WritableStream(ctx *Context, optionalConfig *WritableStreamConfiguration) WritableStream
}

type WritableStreamConfiguration struct{}

type WritableStream interface {
	StreamSink

	// Write should be called by a single goroutine
	Write(ctx *Context, value Value) error

	// WriteChunk should be called by a single goroutine
	WriteChunk(ctx *Context, chunk *DataChunk) error

	Stop()
	IsStopped() bool
}

// A WritableByteStream represents a stream of bytes, ElementsStream implements Value.
type WritableByteStream struct {
	NotClonableMixin

	stopped atomic.Bool

	writeByteToSink func(s *WritableByteStream, b byte) error

	writeBytesToSink func(s *WritableByteStream, p []byte) error
}

func NewWritableByteStream(
	writeByteToSink func(s *WritableByteStream, b byte) error,
	writeChunkToSink func(s *WritableByteStream, p []byte) error,
) *WritableByteStream {
	return &WritableByteStream{writeByteToSink: writeByteToSink, writeBytesToSink: writeChunkToSink}
}

func (s *WritableByteStream) WritableStream(ctx *Context, config *WritableStreamConfiguration) WritableStream {
	return s
}

func (s *WritableByteStream) Write(ctx *Context, v Value) error {
	if s.IsStopped() {
		return ErrStoppedWritableStream
	}

	b, ok := v.(Byte)
	if !ok {
		return ErrInvalidStreamElement
	}

	return s.writeByteToSink(s, byte(b))
}

func (s *WritableByteStream) WriteChunk(ctx *Context, chunk *DataChunk) error {
	if s.IsStopped() {
		return ErrStoppedWritableStream
	}

	data, err := chunk.Data(ctx)
	if err != nil {
		return fmt.Errorf("failed to get data of chunk: %w", err)
	}

	bytes, ok := data.(*ByteSlice)
	if !ok {
		return ErrInvalidStreamChunkData
	}

	return s.WriteBytes(ctx, bytes.Bytes)
}

func (s *WritableByteStream) WriteBytes(ctx *Context, p []byte) error {
	if s.IsStopped() {
		return ErrStoppedWritableStream
	}

	return s.writeBytesToSink(s, p)
}

func (s *WritableByteStream) Stop() {
	//TODO: stop sink ?
	s.stopped.Store(true)
}

func (s *WritableByteStream) IsStopped() bool {
	return s.stopped.Load()
}

func (r *RingBuffer) WritableStream(ctx *Context, config *WritableStreamConfiguration) WritableStream {

	return NewWritableByteStream(
		func(s *WritableByteStream, b byte) error {
			_, err := r.write([]byte{b})
			return err
		},
		func(s *WritableByteStream, p []byte) error {
			_, err := r.write(p)
			return err
		},
	)
}
