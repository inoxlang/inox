package symbolic

import (
	pprint "github.com/inoxlang/inox/internal/pretty_print"
)

var (
	_ = []Readable{(*String)(nil), (*StringConcatenation)(nil)}
)

// A Readable represents a symbolic Readable.
type Readable interface {
	Value
	Reader() *Reader
}

// An AnyReadable represents a symbolic Readable we do not know the concrete type.
type AnyReadable struct {
	_ int
}

func (r *AnyReadable) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	switch val := v.(type) {
	case Readable:
		return true
	default:
		return extData.IsReadable(val)
	}
}

func (r *AnyReadable) PrettyPrint(w PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("readable")
	return
}

func (r *AnyReadable) Reader() *Reader {
	return &Reader{}
}

func (r *AnyReadable) WidestOfType() Value {
	return &AnyReadable{}
}

//

type Reader struct {
	UnassignablePropsMixin
	_ int
}

func (r *Reader) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	switch v.(type) {
	case *Reader:
		return true
	default:
		return false
	}
}

func (reader *Reader) ReadCtx(ctx *Context, b *ByteSlice) (*Int, *Error) {
	return ANY_INT, nil
}

func (reader *Reader) ReadAll() (*ByteSlice, *Error) {
	return &ByteSlice{}, nil
}

func (reader *Reader) Prop(name string) Value {
	method, ok := reader.GetGoMethod(name)
	if !ok {
		panic(FormatErrPropertyDoesNotExist(name, reader))
	}
	return method
}

func (reader *Reader) GetGoMethod(name string) (*GoFunction, bool) {
	switch name {
	case "read":
		return &GoFunction{fn: reader.ReadCtx}, true
	case "read_all":
		return &GoFunction{fn: reader.ReadAll}, true
	}
	return nil, false
}

func (reader *Reader) Reader() *Reader {
	return reader
}

func (Reader) PropertyNames() []string {
	return []string{"read", "read_all"}
}

func (r *Reader) PrettyPrint(w PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("reader")
	return
}

func (r *Reader) WidestOfType() Value {
	return &Reader{}
}

//
