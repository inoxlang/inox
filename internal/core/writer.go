package internal

import (
	"bufio"
	"io"
	"sync"

	"github.com/inoxlang/inox/internal/utils"
)

// A Writable is a Value we can write bytes to thanks to a Writer.
type Writable interface {
	Value
	Writer() *Writer
}

// A Writer is a Value wrapping an io.Writer.
type Writer struct {
	NotClonableMixin
	NoReprMixin

	wrapped      io.Writer
	providedLock *sync.Mutex

	buffered     *bufio.Writer
	current      io.Writer
	totalWritten int // the value is valid only if the wrapper writer is only written to by this writer
}

func WrapWriter(w io.Writer, buffered bool, lock *sync.Mutex) *Writer {
	writer := &Writer{
		wrapped:      w,
		totalWritten: 0,
		providedLock: lock,
	}
	if buffered {
		writer.buffered = bufio.NewWriter(w)
		writer.current = writer.buffered
	} else {
		writer.current = w
	}
	return writer
}

func (w *Writer) Write(p []byte) (n int, err error) {
	if w.providedLock != nil {
		w.providedLock.Lock()
		defer w.providedLock.Unlock()
	}
	return w.writeNoLock(p)
}

func (w *Writer) writeNoLock(p []byte) (n int, err error) {
	n, err = w.current.Write(p)
	w.totalWritten += n
	return n, err
}

func (w *Writer) WriteString(s string) (n int, err error) {
	if w.providedLock != nil {
		w.providedLock.Lock()
		defer w.providedLock.Unlock()
	}
	return w.writeStringNoLock(s)
}

func (w *Writer) writeStringNoLock(s string) (n int, err error) {
	n, err = w.current.Write(utils.StringAsBytes(s))
	w.totalWritten += n
	return n, err
}

func (w *Writer) WriteStrings(s ...string) (n int, err error) {
	if w.providedLock != nil {
		w.providedLock.Lock()
		defer w.providedLock.Unlock()
	}

	total := w.totalWritten
	for _, str := range s {
		if _, err := w.writeStringNoLock(str); err != nil {
			return w.totalWritten - total, err
		}
	}
	return w.totalWritten - total, nil
}

func (w *Writer) writeCtx(ctx *Context, b *ByteSlice) (Int, error) {
	n, err := w.Write(b.Bytes)
	return Int(n), err
}

func (w *Writer) Flush(ctx *Context) error {
	if w.buffered != nil {
		return w.buffered.Flush()
	}
	return nil
}

func (w *Writer) TotalWritten() Int {
	return Int(w.totalWritten)
}

func (w *Writer) Prop(ctx *Context, name string) Value {
	method, ok := w.GetGoMethod(name)
	if !ok {
		panic(FormatErrPropertyDoesNotExist(name, w))
	}
	return method
}

func (*Writer) SetProp(ctx *Context, name string, value Value) error {
	return ErrCannotSetProp
}

func (w *Writer) GetGoMethod(name string) (*GoFunction, bool) {
	switch name {
	case "write":
		return &GoFunction{fn: w.writeCtx}, true
	}
	return nil, false
}

func (w *Writer) Writer() *Writer {
	return w
}

func (Writer) PropertyNames(ctx *Context) []string {
	return []string{"write"}
}
