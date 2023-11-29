package symbolic

import (
	pprint "github.com/inoxlang/inox/internal/prettyprint"
)

var (
	ANY_WRITABLE = &AnyWritable{}
)

// A Writable represents a symbolic Writable.
type Writable interface {
	Value
	Writer() *Writer
}

// An AnyWritable represents a symbolic Writable we do not know the concrete type.
type AnyWritable struct {
	_ int
}

func (r *AnyWritable) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	switch val := v.(type) {
	case Writable:
		return true
	default:
		return extData.IsWritable(val)
	}
}

func (r *AnyWritable) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("writable")
	return
}

func (r *AnyWritable) Writer() *Writer {
	return &Writer{}
}

func (r *AnyWritable) WidestOfType() Value {
	return &AnyWritable{}
}

//

type Writer struct {
	UnassignablePropsMixin
	_ int
}

func (w *Writer) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	switch v.(type) {
	case *Writer:
		return true
	default:
		return false
	}
}

func (w *Writer) write(b *ByteSlice) (*Int, *Error) {
	return ANY_INT, nil
}

func (w *Writer) ReadAll() (*ByteSlice, *Error) {
	return &ByteSlice{}, nil
}

func (w *Writer) Prop(name string) Value {
	method, ok := w.GetGoMethod(name)
	if !ok {
		panic(FormatErrPropertyDoesNotExist(name, w))
	}
	return method
}

func (w *Writer) GetGoMethod(name string) (*GoFunction, bool) {
	switch name {
	case "write":
		return &GoFunction{fn: w.write}, true
	}
	return nil, false
}

func (Writer *Writer) Writer() *Writer {
	return Writer
}

func (Writer) PropertyNames() []string {
	return []string{"write"}
}

func (*Writer) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("writer")
	return
}

func (*Writer) WidestOfType() Value {
	return &Writer{}
}

//
