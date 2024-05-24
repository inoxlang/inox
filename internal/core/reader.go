package core

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/inoxlang/inox/internal/core/symbolic"
)

var (
	_ = []Readable{String(""), GoBytes(nil), (*StringConcatenation)(nil)}

	ErrCannotReadWithNoCopy = errors.New("cannot read with no copy")
)

// A Readable is a Value we can read bytes from thanks to a Reader.
type Readable interface {
	Value
	Reader() *Reader
}

// A Reader is a Value wrapping an io.Reader.
// TODO: close wrapped. when closing Reader
type Reader struct {
	wrapped      io.Reader
	hasAllData   bool
	data         any //[]byte or string
	providedLock *sync.Mutex
}

func WrapReader(wrapped io.Reader, lock *sync.Mutex) *Reader {
	return &Reader{wrapped: wrapped, providedLock: lock}
}

func (r *Reader) Read(p []byte) (n int, err error) {
	if r.providedLock != nil {
		r.providedLock.Lock()
		defer r.providedLock.Unlock()
	}
	return r.wrapped.Read(p)
}

func (r *Reader) ReadCtx(ctx *Context, p *ByteSlice) (*ByteSlice, error) {
	if !p.isDataMutable {
		return nil, ErrModifyImmutable
	}
	n, err := r.Read(p.bytes)
	return &ByteSlice{bytes: p.bytes[:n], isDataMutable: true}, err
}

func (r *Reader) ReadAll() (*ByteSlice, error) {
	b, err := r.ReadAllBytes()
	return &ByteSlice{bytes: b, isDataMutable: true}, err
}

func (r *Reader) ReadAllBytes() ([]byte, error) {
	if r.providedLock != nil {
		r.providedLock.Lock()
		defer r.providedLock.Unlock()
	}

	// TODO: decompose in several reads
	b, err := io.ReadAll(r.wrapped)
	return b, err
}

func (reader *Reader) AlreadyHasAllData() bool {
	return reader.hasAllData
}

// GetBytesDataToNotModify returns all the bytes if they are already available.
// If the bytes are not available the function panics, the returned slice should not be modified.
func (reader *Reader) GetBytesDataToNotModify() []byte {
	if !reader.hasAllData {
		panic(ErrCannotReadWithNoCopy)
	}
	switch v := reader.data.(type) {
	case []byte:
		return v
	case string:
		//TODO: avoid allocation
		return []byte(v)
	default:
		panic(fmt.Errorf("invalid Reader.data: %#v", v))
	}
}

func (reader *Reader) Prop(ctx *Context, name string) Value {
	method, ok := reader.GetGoMethod(name)
	if !ok {
		panic(FormatErrPropertyDoesNotExist(name, reader))
	}
	return method
}

func (*Reader) SetProp(ctx *Context, name string, value Value) error {
	return ErrCannotSetProp
}

func (reader *Reader) GetGoMethod(name string) (*GoFunction, bool) {
	switch name {
	case "read":
		return WrapGoMethod(reader.ReadCtx), true
	case "read_all":
		return WrapGoMethod(reader.ReadAll), true
	}
	return nil, false
}

func (reader *Reader) Reader() *Reader {
	return reader
}

func (Reader) PropertyNames(ctx *Context) []string {
	return symbolic.READER_PRONAMES
}

// ------------------------------------------------------------

func (s String) Reader() *Reader {
	return &Reader{
		wrapped: strings.NewReader(string(s)),
		data:    string(s),
	}
}

func (slice *ByteSlice) Reader() *Reader {
	// only allow if immutable ?
	return &Reader{
		wrapped:    bytes.NewReader(slice.bytes),
		hasAllData: true,
		data:       slice.bytes,
	}
}

func (c *StringConcatenation) Reader() *Reader {
	//TODO: refactor in order to avoid allocating the full string
	s := c.GetOrBuildString()

	return &Reader{
		wrapped: strings.NewReader(s),
		data:    s,
	}
}
