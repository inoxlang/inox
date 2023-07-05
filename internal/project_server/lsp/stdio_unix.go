//go:build unix

package lsp

import (
	"os"

	"github.com/inoxlang/inox/internal/project_server/jsonrpc"
)

func NewStdio() jsonrpc.ReaderWriter {
	return &stdioReaderWriter{
		reader:   os.Stdin,
		writer:   os.Stdout,
		isClosed: false,
	}
}
