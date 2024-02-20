package projectserver

import (
	"errors"
	"fmt"
	"io"
	"log"
	"runtime/debug"
	"strconv"

	"github.com/inoxlang/inox/internal/afs"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/http_ns"
	"github.com/inoxlang/inox/internal/inoxconsts"
	pprint "github.com/inoxlang/inox/internal/prettyprint"
	"github.com/inoxlang/inox/internal/project"
	"github.com/inoxlang/inox/internal/projectserver/jsonrpc"
	"github.com/inoxlang/inox/internal/projectserver/logs"
	"github.com/inoxlang/inox/internal/projectserver/lsp"
	"github.com/inoxlang/inox/internal/utils"
)

const (
	LSP_LOG_SRC                 = "lsp"
	DEFAULT_PROJECT_SERVER_PORT = "8305"
)

var (
	DEFAULT_PROJECT_SERVER_PORT_INT = utils.Must(strconv.Atoi(DEFAULT_PROJECT_SERVER_PORT))
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
	//Setup logs.

	zerologLogger := ctx.NewChildLoggerForInternalSource(LSP_LOG_SRC)
	logger := log.New(zerologLogger, "", 0)
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

	//Configure the LSP server.

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

	if serverConfig.ProdDir != "" {
		ctx.Logger().Debug().Msgf("prod dir is %s", serverConfig.ProdDir)
	}

	//Open the project registry.

	projDir := string(serverConfig.ProjectsDir)
	ctx.Logger().Debug().Msgf("open project registry at %s", projDir)
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

	earlyErr := http_ns.StartDevServer(ctx.BoundChild(), http_ns.DevServerConfig{
		DevServersDir:       devServersDir,
		Port:                inoxconsts.DEV_PORT_0,
		BindToAllInterfaces: serverConfig.ExposeWebServers,
	})

	if earlyErr != nil {
		return fmt.Errorf("failed to start dev server on port %s: %w", inoxconsts.DEV_PORT_0, earlyErr)
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
	}

	logs.Println("LSP server configured, start listening")
	return server.Run()
}
