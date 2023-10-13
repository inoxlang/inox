package symbolic

import (
	"bufio"

	pprint "github.com/inoxlang/inox/internal/pretty_print"
	"github.com/inoxlang/inox/internal/utils"
)

var (
	_ = []Readable{(*String)(nil), (*StringConcatenation)(nil)}
)

// A Readable represents a symbolic Readable.
type Readable interface {
	SymbolicValue
	Reader() *Reader
}

// An AnyReadable represents a symbolic Readable we do not know the concrete type.
type AnyReadable struct {
	_ int
}

func (r *AnyReadable) Test(v SymbolicValue, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	switch val := v.(type) {
	case Readable:
		return true
	default:
		return extData.IsReadable(val)
	}
}

func (r *AnyReadable) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%readable")))
	return
}

func (r *AnyReadable) Reader() *Reader {
	return &Reader{}
}

func (r *AnyReadable) WidestOfType() SymbolicValue {
	return &AnyReadable{}
}

//

type Reader struct {
	UnassignablePropsMixin
	_ int
}

func (r *Reader) Test(v SymbolicValue, state RecTestCallState) bool {
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

func (reader *Reader) Prop(name string) SymbolicValue {
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

func (r *Reader) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%reader")))
	return
}

func (r *Reader) WidestOfType() SymbolicValue {
	return &Reader{}
}

//
