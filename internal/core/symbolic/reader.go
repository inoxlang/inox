package internal

var (
	_ = []Readable{&String{}}
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

func (r *AnyReadable) Test(v SymbolicValue) bool {
	switch val := v.(type) {
	case Readable:
		return true
	default:
		return extData.IsReadable(val)
	}
}

func (r *AnyReadable) Widen() (SymbolicValue, bool) {
	return nil, false
}

func (r *AnyReadable) IsWidenable() bool {
	return false
}

func (r *AnyReadable) String() string {
	return "%readable"
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

func (r *Reader) Test(v SymbolicValue) bool {
	switch v.(type) {
	case *Reader:
		return true
	default:
		return false
	}
}

func (reader *Reader) ReadCtx(ctx *Context, b *ByteSlice) (*Int, *Error) {
	return &Int{}, nil
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
	case "readAll":
		return &GoFunction{fn: reader.ReadAll}, true
	}
	return &GoFunction{}, false
}

func (reader *Reader) Reader() *Reader {
	return reader
}

func (Reader) PropertyNames() []string {
	return []string{"read", "readAll"}
}

func (r *Reader) Widen() (SymbolicValue, bool) {
	return nil, false
}

func (r *Reader) IsWidenable() bool {
	return false
}

func (r *Reader) String() string {
	return "%reader"
}

func (r *Reader) WidestOfType() SymbolicValue {
	return &Reader{}
}

//
