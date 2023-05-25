//go:build unix

package lsp

import (
	"os"

	"github.com/inoxlang/inox/internal/lsp/jsonrpc"
)

func NewStdio() jsonrpc.ReaderWriter {
	return &stdioReaderWriter{
		reader:   os.Stdin,
		writer:   os.Stdout,
		isClosed: false,
	}
}
