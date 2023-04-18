package internal

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
	SymbolicValue
	StreamElement() SymbolicValue
	ChunkedStreamElement() SymbolicValue
}

// An AnyStreamSource represents a symbolic StreamSource we do not know the concrete type.
type AnyStreamSource struct {
	_ int
}

func (r *AnyStreamSource) Test(v SymbolicValue) bool {
	_, ok := v.(StreamSource)

	return ok
}

func (r *AnyStreamSource) Widen() (SymbolicValue, bool) {
	return nil, false
}

func (a *AnyStreamSource) IsWidenable() bool {
	return false
}

func (r *AnyStreamSource) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%stream-source")))
	return
}

func (r *AnyStreamSource) WidestOfType() SymbolicValue {
	return &AnyStreamSource{}
}

func (r *AnyStreamSource) StreamElement() SymbolicValue {
	return ANY
}

func (r *AnyStreamSource) ChunkedStreamElement() SymbolicValue {
	return ANY
}

// An ReadableStream represents a symbolic ReadableStream.
type ReadableStream struct {
	element SymbolicValue //if nil matches any
	_       int
}

// TODO: add chunk argument ?
func NewReadableStream(element SymbolicValue) *ReadableStream {
	return &ReadableStream{element: element}
}

func (r *ReadableStream) Test(v SymbolicValue) bool {
	it, ok := v.(*ReadableStream)
	if !ok {
		return false
	}
	if r.element == nil {
		return true
	}
	return r.element.Test(it.element)
}

func (r *ReadableStream) Widen() (SymbolicValue, bool) {
	if !r.IsWidenable() {
		return nil, false
	}
	return &ReadableStream{}, true
}

func (r *ReadableStream) IsWidenable() bool {
	return r.element != nil
}

func (r *ReadableStream) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%readable-stream")))
	return
}

func (r *ReadableStream) StreamElement() SymbolicValue {
	if r.element == nil {
		return ANY
	}
	return r.element
}

func (r *ReadableStream) ChunkedStreamElement() SymbolicValue {
	return ANY
}

func (r *ReadableStream) WidestOfType() SymbolicValue {
	return &ReadableStream{}
}
