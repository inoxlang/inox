//go:build js

package lsp

import (
	"errors"

	"github.com/inoxlang/inox/internal/projectserver/jsonrpc"
)

func NewStdio() jsonrpc.ReaderWriter {
	panic(errors.New("stdio not available in WASM"))
}
