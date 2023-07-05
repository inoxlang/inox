package jsonrpc

import (
	"fmt"
	"io"
)

var (
	_ MessageReaderWriter = (*FnMessageReaderWriter)(nil)
)

type ReaderWriter interface {
	io.Reader
	io.Writer
	io.Closer
}

type MessageReaderWriter interface {
	//ReadMessage reads an entire message and returns it, the returned bytes should not be modified by the caller.
	ReadMessage() (msg []byte, err error)

	//WriteMessage writes an entire message and returns it, the written bytes should not modified by the implementation.
	WriteMessage(msg []byte) error

	io.Closer
}

type FnMessageReaderWriter struct {
	ReadMessageFn  func() (msg []byte, err error)
	WriteMessageFn func(msg []byte) error
	CloseFn        func() error
}

func (rw FnMessageReaderWriter) ReadMessage() (msg []byte, err error) {
	return rw.ReadMessageFn()
}
func (rw FnMessageReaderWriter) WriteMessage(msg []byte) error {
	return rw.WriteMessageFn(msg)
}

func (rw FnMessageReaderWriter) Close() error {
	return rw.CloseFn()
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
