package project_server

import (
	"errors"
	"io"
	"log"
	"runtime/debug"

	"github.com/inoxlang/inox/internal/afs"
	"github.com/inoxlang/inox/internal/core"
	pprint "github.com/inoxlang/inox/internal/pretty_print"
	"github.com/inoxlang/inox/internal/project_server/jsonrpc"
	"github.com/inoxlang/inox/internal/project_server/logs"
	"github.com/inoxlang/inox/internal/project_server/lsp"
)

const (
	LSP_LOG_SRC = "/lsp"
)

var HOVER_PRETTY_PRINT_CONFIG = &pprint.PrettyPrintConfig{
	MaxDepth: 7,
	Indent:   []byte{' ', ' '},
	Colorize: false,
	Compact:  false,
}

type LSPServerConfiguration struct {
	InternalStdio       *InternalStdio
	Websocket           *WebsocketServerConfiguration
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

type WebsocketServerConfiguration struct {
	Addr                  string
	Certificate           string
	CertificatePrivateKey string
	MaxWebsocketPerIp     int
	BehindCloudProxy      bool
}

func StartLSPServer(ctx *core.Context, serverConfig LSPServerConfiguration) (finalErr error) {
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

	options := &lsp.Config{
		OnSession: serverConfig.OnSession,
	}

	if serverConfig.InternalStdio != nil {

		if serverConfig.Websocket != nil {
			panic(errors.New("invalid LSP options: options for internal STDIO AND Websocket are both provided"))
		}

		options.StdioInput = serverConfig.InternalStdio.StdioInput
		options.StdioOutput = serverConfig.InternalStdio.StdioOutput
	}

	if serverConfig.Websocket != nil {
		if serverConfig.InternalStdio != nil {
			panic(errors.New("invalid LSP options: options for internal STDIO AND Websocket are both provided"))
		}

		options.Network = "wss"
		options.Address = serverConfig.Websocket.Addr
		options.Certificate = serverConfig.Websocket.Certificate
		options.CertificateKey = serverConfig.Websocket.CertificatePrivateKey
		options.MaxWebsocketPerIp = serverConfig.Websocket.MaxWebsocketPerIp
		options.BehindCloudProxy = serverConfig.Websocket.BehindCloudProxy
	}

	if serverConfig.MessageReaderWriter != nil {
		if serverConfig.InternalStdio != nil {
			panic(errors.New("invalid LSP options: MessageReaderWriter AND STDIO both set"))
		}
		if serverConfig.Websocket != nil {
			panic(errors.New("invalid LSP options: MessageReaderWriter AND Websocket both set"))
		}

		options.MessageReaderWriter = serverConfig.MessageReaderWriter
	}

	server := lsp.NewServer(ctx, options)
	registerHandlers(server, serverConfig)

	logs.Println("LSP server configured, start listening")
	return server.Run()
}
