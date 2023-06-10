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

	setupDoneChan := make(chan struct{}, 1)
	inputMessageChannel := make(chan string, 10)
	outputMessageChannel := make(chan string, 10)

	registerCallbacks(inputMessageChannel, outputMessageChannel, setupDoneChan)

	fmt.Println(OUT_PREFIX, "wait for setup() to be called by JS")
	<-setupDoneChan
	close(setupDoneChan)

	printDebug.Invoke(OUT_PREFIX, "create context & state for LSP server")

	serverCtx := core.NewContext(core.ContextConfig{
		Permissions: []core.Permission{
			core.CreateFsReadPerm(core.INITIAL_WORKING_DIR_PATH_PATTERN),
		},
	})
	{
		serverState := core.NewGlobalState(serverCtx)
		serverState.Out = utils.FnWriter{
			WriteFn: func(p []byte) (n int, err error) {
				printDebug.Invoke(OUT_PREFIX, string(p))
				return len(p), nil
			},
		}
		serverState.Logger = zerolog.New(state.Out)
	}

	printDebug.Invoke(OUT_PREFIX, "start LSP server")

	go lsp.StartLSPServer(serverCtx, lsp.LSPServerOptions{
		InoxFS:           true,
		UseContextLogger: true,
		MessageReaderWriter: jsonrpc.FnMessageReaderWriter{
			ReadMessageFn: func() (msg []byte, err error) {
				s, ok := <-inputMessageChannel
				if !ok {
					printDebug.Invoke(OUT_PREFIX, "input message channel is closed")
					return nil, io.EOF
				}
				printDebug.Invoke(OUT_PREFIX, fmt.Sprintf("%d-byte message read from input message channel", len(s)))

				return []byte(s), nil
			},
			WriteMessageFn: func(msg []byte) error {
				outputMessageChannel <- string(msg)
				return nil
			},
			CloseFn: func() error {
				close(inputMessageChannel)
				close(outputMessageChannel)
				return nil
			},
		},
		OnSession: func(rpcCtx *core.Context, s *jsonrpc.Session) error {
			printDebug.Invoke(OUT_PREFIX, "new LSP session")

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

			printDebug.Invoke(OUT_PREFIX, "context of LSP session created")
			return nil
		},
	})

	fmt.Println(OUT_PREFIX, "end of main() reached: block with channel")

	channel := make(chan struct{})
	<-channel
}

func registerCallbacks(inputMessageChannel chan string, outputMessageChannel chan string, setupDoneChan chan struct{}) {
	exports := js.Global().Get("exports")

	exports.Set("write_lsp_message", js.FuncOf(func(this js.Value, args []js.Value) any {
		if printDebug != nil {
			printDebug.Invoke(OUT_PREFIX, "write_lsp_message() called by JS")
		}

		s := args[0].String()
		inputMessageChannel <- s
		return js.ValueOf(nil)
	}))

	exports.Set("read_lsp_message", js.FuncOf(func(this js.Value, args []js.Value) any {
		if printDebug != nil {
			printDebug.Invoke(OUT_PREFIX, "read_lsp_message() called by JS")
		}

		select {
		case msg := <-outputMessageChannel:
			return js.ValueOf(msg)
		case <-time.After(100 * time.Millisecond):
			return js.ValueOf(nil)
		}
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

		setupDoneChan <- struct{}{}

		return js.ValueOf("ok")
	}))

	fmt.Println(OUT_PREFIX, "handlers registered")
}
