package internal

var (
	ANY_WRITABLE = &AnyWritable{}
)

// A Writable represents a symbolic Writable.
type Writable interface {
	SymbolicValue
	Writer() *Writer
}

// An AnyWritable represents a symbolic Writable we do not know the concrete type.
type AnyWritable struct {
	_ int
}

func (r *AnyWritable) Test(v SymbolicValue) bool {
	switch val := v.(type) {
	case Writable:
		return true
	default:
		return extData.IsWritable(val)
	}
}

func (r *AnyWritable) Widen() (SymbolicValue, bool) {
	return nil, false
}

func (r *AnyWritable) IsWidenable() bool {
	return false
}

func (r *AnyWritable) String() string {
	return "Writable"
}

func (r *AnyWritable) Writer() *Writer {
	return &Writer{}
}

func (r *AnyWritable) WidestOfType() SymbolicValue {
	return &AnyWritable{}
}

//

type Writer struct {
	UnassignablePropsMixin
	_ int
}

func (w *Writer) Test(v SymbolicValue) bool {
	switch v.(type) {
	case *Writer:
		return true
	default:
		return false
	}
}

func (w *Writer) write(b *ByteSlice) (*Int, *Error) {
	return &Int{}, nil
}

func (w *Writer) ReadAll() (*ByteSlice, *Error) {
	return &ByteSlice{}, nil
}

func (w *Writer) Prop(name string) SymbolicValue {
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
	return &GoFunction{}, false
}

func (Writer *Writer) Writer() *Writer {
	return Writer
}

func (Writer) PropertyNames() []string {
	return []string{"write"}
}

func (w *Writer) Widen() (SymbolicValue, bool) {
	return nil, false
}

func (*Writer) IsWidenable() bool {
	return false
}

func (*Writer) String() string {
	return "writer"
}

func (*Writer) WidestOfType() SymbolicValue {
	return &Writer{}
}

//
