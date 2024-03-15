package projectserver

import (
	"errors"
	"fmt"
	"io"
	"runtime/debug"
	"time"

	"github.com/inoxlang/inox/internal/afs"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/http_ns"
	"github.com/inoxlang/inox/internal/inoxconsts"
	pprint "github.com/inoxlang/inox/internal/prettyprint"
	"github.com/inoxlang/inox/internal/project"
	"github.com/inoxlang/inox/internal/projectserver/jsonrpc"
	"github.com/inoxlang/inox/internal/projectserver/lsp"
)

const (
	PROJECT_SERVER_LOG_SRC = "project-server"
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
	ProdDir               core.Path //if empty deployment in producation is not allowed
	ProjectsDirFilesystem afs.Filesystem
	ExposeWebServers      bool

	OnSession jsonrpc.SessionCreationCallbackFn
}

type InternalStdio struct {
	StdioInput  io.Reader
	StdioOutput io.Writer
}

type WebsocketServerConfiguration struct {
	Addr                  string //examples: localhost:8305, :8305
	Certificate           string
	CertificatePrivateKey string
	MaxWebsocketPerIp     int
	BehindCloudProxy      bool
}

func StartLSPServer(ctx *core.Context, serverConfig LSPServerConfiguration) (finalErr error) {
	zerologLogger := ctx.NewChildLoggerForInternalSource(PROJECT_SERVER_LOG_SRC)

	defer func() {
		e := recover()

		if e != nil {
			if err, ok := e.(error); ok {
				finalErr = err
			}
			zerologLogger.Println(e, "at", string(debug.Stack()))
		}
	}()

	//Configure the LSP server.

	options := &lsp.Config{
		OnSession: serverConfig.OnSession,
		Logger:    zerologLogger,
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

	if serverConfig.ProdDir != "" {
		zerologLogger.Info().Msgf("prod dir is %s", serverConfig.ProdDir)
	}

	//Open the project registry.

	projDir := string(serverConfig.ProjectsDir)
	zerologLogger.Info().Msgf("open project registry at %s", projDir)
	projectRegistry, err := project.OpenRegistry(projDir, ctx)

	if err != nil {
		return err
	}

	defer projectRegistry.Close(ctx)

	devServersDir, err := projectRegistry.DevServersDir()
	if err != nil {
		return fmt.Errorf("failed to get dev dir of projects: %w", err)
	}

	// Start the development server(s).

	earlyDevServerErrChan := make(chan error, 1)

	for _, port := range []string{inoxconsts.DEV_PORT_0, inoxconsts.DEV_PORT_1, inoxconsts.DEV_PORT_2} {
		go func(port string) {
			earlyErr := http_ns.StartDevServer(ctx.BoundChild(), http_ns.DevServerConfig{
				DevServersDir:       devServersDir,
				Port:                port,
				BindToAllInterfaces: serverConfig.ExposeWebServers,
			})

			if earlyErr != nil {
				earlyDevServerErrChan <- fmt.Errorf("failed to start dev server on port %s: %w", port, earlyErr)
			}

			zerologLogger.Info().Msgf("dev server started on port %s", port)
		}(port)
	}

	select {
	case err := <-earlyDevServerErrChan:
		if err != nil {
			return err
		}
	case <-time.After(2 * http_ns.MAX_DEV_SERVER_STARTUP_PERIOD):
	}

	//Create and start the LSP server.

	server := lsp.NewServer(ctx, options)

	registerStandardMethodHandlers(server, serverConfig)

	if serverConfig.ProjectMode {
		//Register handlers for custom LSP methods.

		registerFilesystemMethodHandlers(server)
		registerProjectMethodHandlers(server, serverConfig, projectRegistry)
		registerProdMethodHandlers(server, serverConfig)
		registerSecretsMethodHandlers(server, serverConfig)
		registerDebugMethodHandlers(server, serverConfig)
		registerLearningMethodHandlers(server)
		registerTestingMethodHandlers(server, serverConfig)
		registerHttpClientMethods(server, serverConfig)
		registerSourceControlMethodHandlers(server, serverConfig)
	}

	zerologLogger.Println("LSP server configured, start listening")
	return server.Run()
}
