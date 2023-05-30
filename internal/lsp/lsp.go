package internal

import (
	"errors"
	"io"
	"log"
	"os"
	"runtime/debug"

	core "github.com/inoxlang/inox/internal/core"
	pprint "github.com/inoxlang/inox/internal/pretty_print"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/rs/zerolog"

	"github.com/inoxlang/inox/internal/lsp/logs"
	"github.com/inoxlang/inox/internal/lsp/lsp"

	"github.com/inoxlang/inox/internal/lsp/lsp/defines"

	_ "net/http/pprof"
)

const (
	JSONRPC_VERSION = "2.0"
)

var HOVER_PRETTY_PRINT_CONFIG = &pprint.PrettyPrintConfig{
	MaxDepth: 7,
	Indent:   []byte{' ', ' '},
	Colorize: false,
	Compact:  false,
}

type LSPServerOptions struct {
	WASM       *WasmOptions
	Websocket  *WebsocketOptions
	Filesystem *Filesystem
}

type WasmOptions struct {
	StdioInput  io.Reader
	StdioOutput io.Writer
	LogOutput   io.Writer
}

type WebsocketOptions struct {
	Addr string
}

func StartLSPServer(ctx *core.Context, opts LSPServerOptions) error {
	//setup logs

	var logOut io.Writer
	var logFile *os.File

	if opts.WASM != nil {
		logOut = opts.WASM.LogOutput
	} else {
		f, err := os.OpenFile("/tmp/.inox-lsp.debug.txt", os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o600)
		if err != nil {
			log.Panicln(err)
		}
		logOut = f
		logFile = f
	}

	logger := log.New(logOut, "", 0)
	logs.Init(logger)

	defer func() {
		e := recover()

		if e != nil {
			logs.Println(e, "at", string(debug.Stack()))
		}

		if logFile != nil {
			logFile.Close()
		}
	}()

	options := &lsp.Options{
		CompletionProvider: &defines.CompletionOptions{
			TriggerCharacters: &[]string{"."},
		},
		TextDocumentSync: defines.TextDocumentSyncKindFull,
	}

	if opts.WASM != nil {

		if opts.Websocket != nil {
			panic(errors.New("invalid LSP options: options for WASM AND Websocket are both provided"))
		}

		options.StdioInput = opts.WASM.StdioInput
		options.StdioOutput = opts.WASM.StdioOutput
	}

	if opts.Websocket != nil {
		if opts.WASM != nil {
			panic(errors.New("invalid LSP options: options for WASM AND Websocket are both provided"))
		}

		options.Network = "wss"
		options.Address = opts.Websocket.Addr
	}

	server := lsp.NewServer(ctx, options)

	state := core.NewGlobalState(ctx)
	state.Logger = zerolog.New(utils.FnWriter{
		WriteFn: func(p []byte) (n int, err error) {
			logs.Println(utils.BytesAsString(p))
			return len(p), nil
		},
	})

	registerHandlers(server, opts.Filesystem, ctx)
	return server.Run()
}
