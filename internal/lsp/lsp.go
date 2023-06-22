package internal

import (
	"errors"
	"io"
	"log"
	"runtime/debug"

	"github.com/inoxlang/inox/internal/afs"
	core "github.com/inoxlang/inox/internal/core"
	pprint "github.com/inoxlang/inox/internal/pretty_print"

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
	InternalStdio       *InternalStdio
	Websocket           *WebsocketOptions
	MessageReaderWriter jsonrpc.MessageReaderWriter
	UseContextLogger    bool

	ProjectMode           bool
	ProjectsDir           core.Path
	ProjectsDirFilesystem afs.Filesystem

	OnSession jsonrpc.SessionCreationCallbackFn
}

type InternalStdio struct {
	StdioInput  io.Reader
	StdioOutput io.Writer
}

type WebsocketOptions struct {
	Addr                  string
	Certificate           string
	CertificatePrivateKey string
}

func StartLSPServer(ctx *core.Context, opts LSPServerOptions) (finalErr error) {
	//setup logs

	logOut := ctx.Logger().With().Str(core.SOURCE_LOG_FIELD_NAME, LSP_LOG_SRC).Logger()
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
		options.WebsocketCertificate = opts.Websocket.Certificate
		options.WebsocketCertificateKey = opts.Websocket.CertificatePrivateKey
	}

	if opts.MessageReaderWriter != nil {
		if opts.InternalStdio != nil {
			panic(errors.New("invalid LSP options: MessageReaderWriter AND STDIO both set"))
		}
		if opts.Websocket != nil {
			panic(errors.New("invalid LSP options: MessageReaderWriter AND Websocket both set"))
		}

		options.MessageReaderWriter = opts.MessageReaderWriter
	}

	server := lsp.NewServer(ctx, options)
	registerHandlers(server, opts)

	logs.Println("LSP server configured, start listening")
	return server.Run()
}
