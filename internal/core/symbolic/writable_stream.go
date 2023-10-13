package symbolic

import (
	"bufio"

	pprint "github.com/inoxlang/inox/internal/pretty_print"
	"github.com/inoxlang/inox/internal/utils"
)

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

func (r *AnyStreamSink) Test(v SymbolicValue, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	_, ok := v.(StreamSink)

	return ok
}

func (r *AnyStreamSink) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%stream-sink")))
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

func (r *WritableStream) Test(v SymbolicValue, state RecTestCallState) bool {
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

func (r *WritableStream) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%writable-stream")))
	return
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
