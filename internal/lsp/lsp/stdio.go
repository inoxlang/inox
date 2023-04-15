package lsp

import (
	"io"
	"os"

	"github.com/inox-project/inox/internal/lsp/jsonrpc"
)

type stdioReaderWriter struct {
	reader   io.Reader
	writer   io.Writer
	isClosed bool
}

func NewStdio() jsonrpc.ReaderWriter {
	return &stdioReaderWriter{
		reader:   os.Stdin,
		writer:   os.Stdout,
		isClosed: false,
	}
}

func (s *stdioReaderWriter) Read(p []byte) (n int, err error) {
	if s.isClosed {
		return 0, io.EOF
	}
	return s.reader.Read(p)
}

func (s *stdioReaderWriter) Write(p []byte) (n int, err error) {
	if s.isClosed {
		return 0, io.EOF
	}
	return s.writer.Write(p)
}

func (s *stdioReaderWriter) Close() error {
	s.isClosed = true
	return nil
}
