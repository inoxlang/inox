package cloudproxy

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/inoxlang/inox/internal/afs"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/http_ns"
	"github.com/inoxlang/inox/internal/globals/net_ns"
	"github.com/inoxlang/inox/internal/inoxd/cloud/account"
	nettypes "github.com/inoxlang/inox/internal/net_types"
	"github.com/inoxlang/inox/internal/project_server"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/rs/zerolog"
)

const (
	CLOUD_PROXY_SUBCMD_NAME                = "cloud-proxy"
	ACCOUNT_TOKEN_HEADER_NAME              = "X-Account-Token"
	ACCOUT_MANAGEMENT_LOG_SRC              = "account"
	ACCOUNT_REGISTRATION_URL_PATH          = "/register-account"
	ACCOUNT_REGISTRATION_HOSTER_PARAM_NAME = "hoster"
)

type CloudProxyArgs struct {
	Config     CloudProxyConfig
	OutW, ErrW io.Writer
	GoContext  context.Context
	Filesystem afs.Filesystem
}

type CloudProxyConfig struct {
	AnonymousAccountDatabasePath string `json:"anonAccountDBPath,omitempty"`
}

func Run(args CloudProxyArgs) error {
	if args.Config.AnonymousAccountDatabasePath == "" {
		args.Config.AnonymousAccountDatabasePath = account.DEFAULT_ANON_ACCOUNT_STORE_PATH
	}

	config := args.Config
	outW := args.OutW
	//errW := args.ErrW
	dbPath := core.Path(config.AnonymousAccountDatabasePath)

	addr := "localhost:" + project_server.DEFAULT_PROJECT_SERVER_PORT
	host := core.Host("wss://" + addr)

	ctx, topCtx := createContext(host, args)
	defer topCtx.CancelGracefully()

	wsServer, err := net_ns.NewWebsocketServer(ctx)

	if err != nil {
		return err
	}

	accountDB, err := account.OpenAnonymousAccountDatabase(ctx, dbPath, ctx.GetFileSystem())

	if err != nil {
		return err
	}

	accountManagementLogger := ctx.Logger().With().Str(core.SOURCE_LOG_FIELD_NAME, ACCOUT_MANAGEMENT_LOG_SRC).Logger()

	httpServer, err := http_ns.NewGolangHttpServer(ctx, http_ns.GolangHttpServerConfig{
		Addr: addr,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			if r.URL.Path == ACCOUNT_REGISTRATION_URL_PATH {
				hoster := r.URL.Query().Get("hoster")
				if hoster == "" {
					w.WriteHeader(http.StatusBadRequest)
					return
				}

				socket, err := wsServer.UpgradeGoValues(w, r, allowConnection)

				if err != nil {
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				go registerAccount(ctx, socket, hoster, accountDB, accountManagementLogger)
				return
			}

			if r.URL.Path != "/" {
				return
			}

			//else we check the account token and connect to the project server.

			tokenHeaderValues, ok := r.Header[ACCOUNT_TOKEN_HEADER_NAME]
			if !ok || len(tokenHeaderValues) == 0 {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}

			account, err := accountDB.GetAccount(ctx, tokenHeaderValues[0])
			if err != nil {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}

			accountManagementLogger.Info().Msg("account successfully connected: " + account.ULID)

			id := account.ULID
			_ = id
			//TODO: connect to account-specific project server.

			_, err = wsServer.UpgradeGoValues(w, r, allowConnection)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
			}
		}),
	})

	if err != nil {
		return err
	}

	ctx.OnGracefulTearDown(func(ctx *core.Context) error {
		return httpServer.Shutdown(ctx)
	})

	ctx.OnDone(func(timeoutCtx context.Context) error {
		return httpServer.Close()
	})

	fmt.Fprintf(outW, "start cloud proxy HTTPS server listening on %s\n", addr)

	err = httpServer.ListenAndServeTLS("", "")
	if err != nil {
		return fmt.Errorf("failed to create HTTPS server for cloud proxy: %w", err)
	}

	return nil
}

func allowConnection(remoteAddrPort nettypes.RemoteAddrWithPort, remoteAddr nettypes.RemoteIpAddr, currentConns []*net_ns.WebsocketConnection) error {
	return nil
}

func registerAccount(
	ctx *core.Context,
	socket *net_ns.WebsocketConnection,
	hoster string,
	accountDB *account.AnonymousAccountDatabase,
	logger zerolog.Logger,
) {
	defer utils.Recover()
	defer socket.Close()

	//Bidirectionnial connection used to interactively register the account with the user.
	//CreateAnonymousAccountInteractively will show information to the user and the user will send several inputs.
	conn := &account.Connection{
		PrintFn: func(text string) {
			socket.WriteMessage(ctx, net_ns.WebsocketTextMessage, []byte(text))
		},
		ReadChan: make(chan string, 5),
	}

	receivedMsgChan := make(chan net_ns.WebsocketMessageChanItem, 10)
	err := socket.StartReadingAllMessagesIntoChan(ctx, receivedMsgChan)
	if err != nil {
		//TODO: log errors (make sure to not write logs locally in order to not run ouf space).
		return
	}

	//goroutine that writes to conn.ReadChan the text messages from the websocket.
	go func() {
		defer utils.Recover()
		defer close(conn.ReadChan)

		for item := range receivedMsgChan {
			if item.Error != nil {
				break
			}
			if item.Type != net_ns.WebsocketTextMessage {
				continue
			}
			select {
			case conn.ReadChan <- string(item.Payload):
			default:
				//give up
				return
			}
		}
	}()

	err = account.CreateAnonymousAccountInteractively(ctx, hoster, conn, accountDB)
	if err != nil {
		logger.Err(err).Send()
	}
}
