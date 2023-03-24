package utils

import (
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
