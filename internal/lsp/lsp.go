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

	"github.com/inoxlang/inox/internal/lsp/jsonrpc"
	"github.com/inoxlang/inox/internal/lsp/logs"
	"github.com/inoxlang/inox/internal/lsp/lsp"

	"github.com/inoxlang/inox/internal/lsp/lsp/defines"

	_ "net/http/pprof"
)

const (
	JSONRPC_VERSION = "2.0"
	LSP_LOG_SRC     = "/lsp"
)

var HOVER_PRETTY_PRINT_CONFIG = &pprint.PrettyPrintConfig{
	MaxDepth: 7,
	Indent:   []byte{' ', ' '},
	Colorize: false,
	Compact:  false,
}

type LSPServerOptions struct {
	InternalStdio    *InternalStdio
	Websocket        *WebsocketOptions
	Filesystem       *Filesystem
	UseContextLogger bool

	OnSession jsonrpc.SessionCreationCallbackFn
}

type InternalStdio struct {
	StdioInput  io.Reader
	StdioOutput io.Writer
	LogOutput   io.Writer
}

type WebsocketOptions struct {
	Addr string
}

func StartLSPServer(ctx *core.Context, opts LSPServerOptions) (finalErr error) {
	//setup logs

	var logOut io.Writer
	var logFile *os.File

	if opts.InternalStdio != nil {
		logOut = opts.InternalStdio.LogOutput
	} else if opts.UseContextLogger {
		logOut = ctx.Logger().With().Str(core.SOURCE_LOG_FIELD_NAME, LSP_LOG_SRC).Logger()
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
			if err, ok := e.(error); ok {
				finalErr = err
			}
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
		OnSession:        opts.OnSession,
	}

	if opts.InternalStdio != nil {

		if opts.Websocket != nil {
			panic(errors.New("invalid LSP options: options for internal STDIO AND Websocket are both provided"))
		}

		options.StdioInput = opts.InternalStdio.StdioInput
		options.StdioOutput = opts.InternalStdio.StdioOutput
	}

	if opts.Websocket != nil {
		if opts.InternalStdio != nil {
			panic(errors.New("invalid LSP options: options for internal STDIO AND Websocket are both provided"))
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

	registerHandlers(server, ctx)
	return server.Run()
}
