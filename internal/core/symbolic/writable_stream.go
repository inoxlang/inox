package internal

var (
	ANY_STREAM_SINK = &AnyStreamSink{}
	_               = []StreamSink{ANY_STREAM_SINK, &WritableStream{}}
)

// An StreamSink represents a symbolic StreamSink.
type StreamSink interface {
	SymbolicValue
	WritableStreamElement() SymbolicValue
	ChunkedWritableStreamElement() SymbolicValue
}

// An AnyStreamSink represents a symbolic StreamSink we do not know the concrete type.
type AnyStreamSink struct {
	_ int
}

func (r *AnyStreamSink) Test(v SymbolicValue) bool {
	_, ok := v.(StreamSink)

	return ok
}

func (r *AnyStreamSink) Widen() (SymbolicValue, bool) {
	return nil, false
}

func (a *AnyStreamSink) IsWidenable() bool {
	return false
}

func (r *AnyStreamSink) String() string {
	return "stream-sink"
}

func (r *AnyStreamSink) WidestOfType() SymbolicValue {
	return ANY_STREAM_SINK
}

func (r *AnyStreamSink) WritableStreamElement() SymbolicValue {
	return ANY
}

func (r *AnyStreamSink) ChunkedWritableStreamElement() SymbolicValue {
	return ANY
}

// An WritableStream represents a symbolic WritableStream.
type WritableStream struct {
	element SymbolicValue //if nil matches any
	_       int
}

// TODO: add chunk argument ?
func NewWritableStream(element SymbolicValue) *WritableStream {
	return &WritableStream{element: element}
}

func (r *WritableStream) Test(v SymbolicValue) bool {
	it, ok := v.(*WritableStream)
	if !ok {
		return false
	}
	if r.element == nil {
		return true
	}
	return r.element.Test(it.element)
}

func (r *WritableStream) Widen() (SymbolicValue, bool) {
	if !r.IsWidenable() {
		return nil, false
	}
	return &WritableStream{}, true
}

func (r *WritableStream) IsWidenable() bool {
	return r.element != nil
}

func (r *WritableStream) String() string {
	return "%writable-stream"
}

func (r *WritableStream) WritableStreamElement() SymbolicValue {
	if r.element == nil {
		return ANY
	}
	return r.element
}

func (r *WritableStream) ChunkedWritableStreamElement() SymbolicValue {
	return ANY
}

func (r *WritableStream) WidestOfType() SymbolicValue {
	return &WritableStream{}
}
