package symbolic

import (
	"bufio"

	pprint "github.com/inoxlang/inox/internal/pretty_print"
	"github.com/inoxlang/inox/internal/utils"
)

var (
	ANY_STREAM_SOURCE = &AnyStreamSource{}
	_                 = []StreamSource{ANY_STREAM_SOURCE, &ReadableStream{}}
)

// An StreamSource represents a symbolic StreamSource.
type StreamSource interface {
	Value
	StreamElement() Value
	ChunkedStreamElement() Value
}

// An AnyStreamSource represents a symbolic StreamSource we do not know the concrete type.
type AnyStreamSource struct {
	_ int
}

func (r *AnyStreamSource) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	_, ok := v.(StreamSource)

	return ok
}

func (r *AnyStreamSource) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%stream-source")))
	return
}

func (r *AnyStreamSource) WidestOfType() Value {
	return &AnyStreamSource{}
}

func (r *AnyStreamSource) StreamElement() Value {
	return ANY
}

func (r *AnyStreamSource) ChunkedStreamElement() Value {
	return ANY
}

// An ReadableStream represents a symbolic ReadableStream.
type ReadableStream struct {
	element Value //if nil matches any
	_       int
}

// TODO: add chunk argument ?
func NewReadableStream(element Value) *ReadableStream {
	return &ReadableStream{element: element}
}

func (r *ReadableStream) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	it, ok := v.(*ReadableStream)
	if !ok {
		return false
	}
	if r.element == nil {
		return true
	}
	return r.element.Test(it.element, state)
}

func (r *ReadableStream) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%readable-stream")))
	return
}

func (r *ReadableStream) StreamElement() Value {
	if r.element == nil {
		return ANY
	}
	return r.element
}

func (r *ReadableStream) ChunkedStreamElement() Value {
	return ANY
}

func (r *ReadableStream) WidestOfType() Value {
	return &ReadableStream{}
}
