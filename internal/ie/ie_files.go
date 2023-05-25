package internal

import (
	_ "embed"
)

//go:embed browser-lsp-server.wasm
var INOX_WASM []byte
