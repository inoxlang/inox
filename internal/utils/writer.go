package utils

import (
	"io"
	"testing"
)

type TestWriter struct {
	T *testing.T
}

func (w *TestWriter) Write(p []byte) (n int, err error) {
	w.T.Log(string(p))
	return len(p), nil
}

type FnWriter struct {
	fn func(p []byte) (n int, err error)
}

func (writer FnWriter) Write(p []byte) (n int, err error) {
	return writer.fn(p)
}

func WriteMany[W io.Writer](w W, slices ...[]byte) error {
	for _, s := range slices {
		if _, err := w.Write(s); err != nil {
			return err
		}
	}
	return nil
}
