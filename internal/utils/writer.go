package utils

import (
	"io"
	"testing"
)

var (
	_ = []io.Writer{FnWriter{}, FnReaderWriter{}}
	_ = []io.Reader{FnReaderWriter{}}
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
