package symbolic

import (
	pprint "github.com/inoxlang/inox/internal/pretty_print"
)

var (
	ANY_STREAM_SINK = &AnyStreamSink{}
	_               = []StreamSink{ANY_STREAM_SINK, &WritableStream{}}
)

// An StreamSink represents a symbolic StreamSink.
type StreamSink interface {
	Value
	WritableStreamElement() Value
	ChunkedWritableStreamElement() Value
}

// An AnyStreamSink represents a symbolic StreamSink we do not know the concrete type.
type AnyStreamSink struct {
	_ int
}

func (r *AnyStreamSink) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	_, ok := v.(StreamSink)

	return ok
}

func (r *AnyStreamSink) PrettyPrint(w PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("stream-sink")
}

func (r *AnyStreamSink) WidestOfType() Value {
	return ANY_STREAM_SINK
}

func (r *AnyStreamSink) WritableStreamElement() Value {
	return ANY
}

func (r *AnyStreamSink) ChunkedWritableStreamElement() Value {
	return ANY
}

// An WritableStream represents a symbolic WritableStream.
type WritableStream struct {
	element Value //if nil matches any
	_       int
}

// TODO: add chunk argument ?
func NewWritableStream(element Value) *WritableStream {
	return &WritableStream{element: element}
}

func (r *WritableStream) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	it, ok := v.(*WritableStream)
	if !ok {
		return false
	}
	if r.element == nil {
		return true
	}
	return r.element.Test(it.element, state)
}

func (r *WritableStream) PrettyPrint(w PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("writable-stream")
	return
}

func (r *WritableStream) WritableStreamElement() Value {
	if r.element == nil {
		return ANY
	}
	return r.element
}

func (r *WritableStream) ChunkedWritableStreamElement() Value {
	return ANY
}

func (r *WritableStream) WidestOfType() Value {
	return &WritableStream{}
}
