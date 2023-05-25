//go:build js

package main

import (
	"fmt"
	"os"
	"syscall/js"

	core "github.com/inoxlang/inox/internal/core"
	lsp "github.com/inoxlang/inox/internal/lsp"
	"github.com/inoxlang/inox/internal/utils"
)

const (
	LSP_INPUT_BUFFER_SIZE  = 5_000_000
	LSP_OUTPUT_BUFFER_SIZE = 5_000_000
	OUT_PREFIX             = "[lsp server module]"
)

func main() {
	fmt.Println(OUT_PREFIX, "start")
	ctx := core.NewContext(core.ContextConfig{})

	lspInput := core.NewRingBuffer(ctx, LSP_INPUT_BUFFER_SIZE)
	lspOutput := core.NewRingBuffer(ctx, LSP_OUTPUT_BUFFER_SIZE)
	registerCallbacks(lspInput, lspOutput)

	fmt.Println(OUT_PREFIX, "start server")

	lsp.StartLSPServer(lsp.LSPServerOptions{
		WASM: &lsp.WasmOptions{
			StdioInput:  lspInput,
			StdioOutput: lspOutput,
			LogOutput:   os.Stdout,
		},
	})
}

func registerCallbacks(lspInput *core.RingBuffer, lspOutput *core.RingBuffer) {
	global := js.Global()
	global.Set("write_lsp_input", js.FuncOf(func(this js.Value, args []js.Value) any {
		fmt.Println(OUT_PREFIX, "write_lsp_input() called by JS")

		s := args[0].String()
		lspInput.Write(utils.StringAsBytes(s))
		return js.ValueOf(nil)
	}))

	global.Set("read_lsp_output", js.FuncOf(func(this js.Value, args []js.Value) any {
		fmt.Println(OUT_PREFIX, "read_lsp_output() called by JS")

		b := lspOutput.ReadableBytesCopy()
		return js.ValueOf(string(b))
	}))

	global.Set("setup", js.FuncOf(func(this js.Value, args []js.Value) any {
		fmt.Println(OUT_PREFIX, "setup() called by JS")

		IWD := args[0].Get(core.INITIAL_WORKING_DIR_VARNAME).String()

		core.SetInitialWorkingDir(func() (string, error) {
			return IWD, nil
		})

		b := lspOutput.ReadableBytesCopy()
		return js.ValueOf(string(b))
	}))
}
