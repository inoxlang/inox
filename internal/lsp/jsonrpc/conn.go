package jsonrpc

import (
	"fmt"
	"io"
)

type ReaderWriter interface {
	io.Reader
	io.Writer
	io.Closer
}
type CloserReader interface {
	io.Reader
	io.Closer
}
type fakeCloseReader struct {
	io.Reader
}

func (f *fakeCloseReader) Close() error {
	return nil
}

func NewFakeCloserReader(r io.Reader) CloserReader {
	return &fakeCloseReader{r}
}

type CloserWriter interface {
	io.Writer
	io.Closer
}

type fakeCloseWriter struct {
	io.Writer
}

func (f *fakeCloseWriter) Close() error {
	return nil
}

func NewFakeCloserWriter(w io.Writer) CloserWriter {
	return &fakeCloseWriter{w}
}

// The connection of rpc, not limited to net.Conn
type Conn struct {
	reader CloserReader
	writer CloserWriter
}

func NewNotCloseConn(reader io.Reader, writer io.Writer) *Conn {
	return &Conn{reader: NewFakeCloserReader(reader), writer: NewFakeCloserWriter(writer)}
}

func NewConn(reader CloserReader, writer CloserWriter) *Conn {
	return &Conn{reader: reader, writer: writer}
}

func (c *Conn) Write(p []byte) (n int, err error) {
	return c.writer.Write(p)
}
func (c *Conn) Read(p []byte) (n int, err error) {
	return c.reader.Read(p)
}

func (c *Conn) Close() error {
	var err1 = c.reader.Close()
	var err2 = c.reader.Close()
	if err1 == nil && err2 == nil {
		return nil
	}
	if err1 == nil {
		return err2
	}
	if err2 == nil {
		return err1
	}
	return fmt.Errorf("two errors, err1: %v, err2: %v", err1, err2)
}

