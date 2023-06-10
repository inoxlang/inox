//go:build js

package main

import (
	"fmt"
	"io"
	"syscall/js"
	"time"

	"github.com/inoxlang/inox/internal/core"
	_ "github.com/inoxlang/inox/internal/globals"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/inoxlang/inox/internal/lsp/jsonrpc"
	"github.com/rs/zerolog"

	lsp "github.com/inoxlang/inox/internal/lsp"
	"github.com/inoxlang/inox/internal/utils"
)

const (
	LSP_INPUT_BUFFER_SIZE  = 5_000_000
	LSP_OUTPUT_BUFFER_SIZE = 5_000_000
	OUT_PREFIX             = "[vscode-inox]"
)

var printDebug *js.Value

func main() {
	ctx := core.NewContext(core.ContextConfig{})
	state := core.NewGlobalState(ctx)
	state.Out = utils.FnWriter{
		WriteFn: func(p []byte) (n int, err error) {
			if printDebug != nil {
				printDebug.Invoke(OUT_PREFIX, string(p))
			}
			return len(p), nil
		},
	}
	state.Logger = zerolog.New(state.Out)

	pauseChan := make(chan struct{})
	setupDoneChan := make(chan struct{}, 1)

	lspInput := core.NewRingBuffer(ctx, LSP_INPUT_BUFFER_SIZE)
	lspInputWriter := utils.FnReaderWriter{
		WriteFn: func(p []byte) (n int, err error) {
			if printDebug != nil {
				printDebug.Invoke(OUT_PREFIX, "resume reading because we are going to write")
			}
			select {
			case <-pauseChan:
			case <-time.After(100 * time.Millisecond):
			}
			if printDebug != nil {
				printDebug.Invoke(OUT_PREFIX, "write LSP input")
			}
			return lspInput.Write(p)
		},
		ReadFn: func(p []byte) (n int, err error) {
			if lspInput.ReadableCount(ctx) == 0 {
				if printDebug != nil {
					printDebug.Invoke(OUT_PREFIX, "pause read call because there is nothing to read")
				}

				pauseChan <- struct{}{}
			}

			if printDebug != nil {
				printDebug.Invoke(OUT_PREFIX, "read LSP input")
			}

			return lspInput.Read(p)
		},
	}

	lspOuput := core.NewRingBuffer(ctx, LSP_OUTPUT_BUFFER_SIZE)
	registerCallbacks(lspInputWriter, lspOuput, setupDoneChan)

	fmt.Println(OUT_PREFIX, "wait for setup() to be called by JS")
	<-setupDoneChan
	close(setupDoneChan)

	fmt.Println(OUT_PREFIX, "start LSP server")

	serverCtx := ctx.BoundChild()
	go lsp.StartLSPServer(serverCtx, lsp.LSPServerOptions{
		InoxFS: true,
		InternalStdio: &lsp.InternalStdio{
			StdioInput:  lspInputWriter,
			StdioOutput: lspOuput,
			LogOutput: utils.FnWriter{
				WriteFn: func(p []byte) (n int, err error) {
					if printDebug != nil {
						printDebug.Invoke(OUT_PREFIX, utils.BytesAsString(p))
					}
					return len(p), nil
				},
			},
		},
		OnSession: func(rpcCtx *core.Context, s *jsonrpc.Session) error {
			printDebug.Invoke("new LSP session")

			mainFs := fs_ns.NewMemFilesystem(lsp.DEFAULT_MAX_IN_MEM_FS_STORAGE_SIZE)
			fls := lsp.NewFilesystem(mainFs, nil)

			file := utils.Must(fls.Create("/main.ix"))
			utils.Must(file.Write([]byte("manifest {\n\n}")))
			utils.PanicIfErr(file.Close())

			sessionCtx := core.NewContext(core.ContextConfig{
				Permissions:          rpcCtx.GetGrantedPermissions(),
				ForbiddenPermissions: rpcCtx.GetForbiddenPermissions(),

				ParentContext: rpcCtx,
				Filesystem:    fls,
			})
			tempState := core.NewGlobalState(sessionCtx)
			tempState.Logger = state.Logger
			tempState.Out = state.Out
			s.SetContextOnce(sessionCtx)

			return nil
		},
	})

	fmt.Println(OUT_PREFIX, "end of main() reached: block with channel")

	channel := make(chan struct{})
	<-channel
}

func registerCallbacks(lspInput io.ReadWriter, lspOutput *core.RingBuffer, setupDoneChan chan struct{}) {
	exports := js.Global().Get("exports")

	exports.Set("write_lsp_input", js.FuncOf(func(this js.Value, args []js.Value) any {
		if printDebug != nil {
			printDebug.Invoke(OUT_PREFIX, "write_lsp_input() called by JS")
		}

		s := args[0].String()
		lspInput.Write(utils.StringAsBytes(s))
		return js.ValueOf(nil)
	}))

	exports.Set("read_lsp_output", js.FuncOf(func(this js.Value, args []js.Value) any {
		if printDebug != nil {
			printDebug.Invoke(OUT_PREFIX, "read_lsp_output() called by JS")
		}

		b := make([]byte, LSP_OUTPUT_BUFFER_SIZE)
		n, err := lspOutput.Read(b)
		if err != nil && printDebug != nil {
			printDebug.Invoke(OUT_PREFIX, "read_lsp_output():", err.Error())
		}
		return js.ValueOf(string(b[:n]))
	}))

	exports.Set("setup", js.FuncOf(func(this js.Value, args []js.Value) any {
		fmt.Println(OUT_PREFIX, "setup() called by JS")

		if printDebug != nil {
			fmt.Println("setup() already called by JS !")
			return js.ValueOf("")
		}

		IWD := args[0].Get(core.INITIAL_WORKING_DIR_VARNAME).String()
		debug := args[0].Get("print_debug")
		printDebug = &debug

		core.SetInitialWorkingDir(func() (string, error) {
			return IWD, nil
		})

		b := lspOutput.ReadableBytesCopy()

		setupDoneChan <- struct{}{}

		return js.ValueOf(string(b))
	}))

	fmt.Println(OUT_PREFIX, "handlers registered")
}
