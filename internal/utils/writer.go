package utils

import (
	"errors"
	"io"
	"testing"
)

var (
	_ = []io.Writer{FnWriter{}, FnReaderWriter{}}
	_ = []io.Reader{FnReaderWriter{}}
	_ = []io.ReadWriteCloser{FnReaderWriterCloser{}}
)

type TestWriter struct {
	T *testing.T
}

func (w *TestWriter) Write(p []byte) (n int, err error) {
	w.T.Log(string(p))
	return len(p), nil
}

type FnWriter struct {
	WriteFn func(p []byte) (n int, err error)
}

func (writer FnWriter) Write(p []byte) (n int, err error) {
	return writer.WriteFn(p)
}

type FnReader func(p []byte) (n int, err error)

func (fn FnReader) Read(p []byte) (n int, err error) {
	return fn(p)
}

type FnReaderWriter struct {
	WriteFn func(p []byte) (n int, err error)
	ReadFn  func(p []byte) (n int, err error)
}

func (w FnReaderWriter) Write(p []byte) (n int, err error) {
	return w.WriteFn(p)
}

func (w FnReaderWriter) Read(p []byte) (n int, err error) {
	return w.ReadFn(p)
}

type FnReaderWriterCloser struct {
	WriteFn func(p []byte) (n int, err error)
	ReadFn  func(p []byte) (n int, err error)
	CloseFn func() error
}

func (w FnReaderWriterCloser) Write(p []byte) (n int, err error) {
	return w.WriteFn(p)
}

func (w FnReaderWriterCloser) Read(p []byte) (n int, err error) {
	return w.ReadFn(p)
}

func (w FnReaderWriterCloser) Close() error {
	return w.CloseFn()
}

func WriteMany[W io.Writer](w W, slices ...[]byte) error {
	for _, s := range slices {
		if _, err := w.Write(s); err != nil {
			return err
		}
	}
	return nil
}

func MustWriteMany[W io.Writer](w W, slices ...[]byte) {
	PanicIfErr(WriteMany(w, slices...))
}

var ErrOutOfSpace = errors.New("out of space")

// FixedBufferWriter writes data in the wrapper byte slice, it never allocates a new slice.
// ErrOutOfSpace is returned if there not enough space.
type FixedBufferWriter []byte

func (w *FixedBufferWriter) Write(p []byte) (int, error) {
	currentLen := len(*w)
	available := cap(*w) - currentLen
	availableStart := currentLen

	if available < len(p) {
		return 0, ErrOutOfSpace
	}
	n := copy((*w)[availableStart:], p)
	newLen := currentLen + n
	*w = (*w)[:newLen]
	return n, nil
}
