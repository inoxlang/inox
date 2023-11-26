package cloudproxy

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/inoxlang/inox/internal/afs"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/inoxlang/inox/internal/globals/http_ns"
	"github.com/inoxlang/inox/internal/globals/net_ns"
	"github.com/inoxlang/inox/internal/inoxd/cloud/account"
	"github.com/inoxlang/inox/internal/inoxd/consts"
	"github.com/inoxlang/inox/internal/inoxprocess"
	nettypes "github.com/inoxlang/inox/internal/net_types"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/rs/zerolog"
)

const (
	CLOUD_PROXY_SUBCMD_NAME   = "cloud-proxy"
	ACCOUNT_TOKEN_HEADER_NAME = "X-Account-Token"

	ACCOUT_MANAGEMENT_LOG_SRC = "/account"
	PROXY_LOG_SRC             = "/cloud-proxy"

	ACCOUNT_REGISTRATION_URL_PATH          = "/register-account"
	ACCOUNT_REGISTRATION_HOSTER_PARAM_NAME = "hoster"
)

type CloudProxyArgs struct {
	Config                CloudProxyConfig
	OutW, ErrW            io.Writer
	GoContext             context.Context
	Filesystem            afs.Filesystem
	RestrictProcessAccess bool
}

type CloudProxyConfig struct {
	CloudDataDir string `json:"dataDir"`
	//should be in DataDir, if not set defaults to <DATA DIR>/<DEFAULT_ANON_ACCOUNT_DB_BASENAME>
	AnonymousAccountDatabasePath string `json:"anonAccountDBPath,omitempty"`
	Port                         int    `json:"port"`
}

func Run(args CloudProxyArgs) error {
	if args.Config.CloudDataDir == "" {
		return errors.New("invalid cloud-proxy configuration: missing cloud data dir")
	}

	if args.Config.AnonymousAccountDatabasePath == "" {
		args.Config.AnonymousAccountDatabasePath = filepath.Join(args.Config.CloudDataDir, consts.DEFAULT_ANON_ACCOUNT_DB_BASENAME)
	}

	if !strings.HasPrefix(args.Config.AnonymousAccountDatabasePath, core.AppendTrailingSlashIfNotPresent(args.Config.CloudDataDir)) {
		return errors.New("invalid cloud-proxy configuration: the anonymous account database should be located in the cloud data directory")
	}

	if args.Config.Port == 0 {
		return errors.New("invalid cloud-proxy configuration: missing port")
	}

	if args.Filesystem == nil {
		args.Filesystem = fs_ns.GetOsFilesystem()
	}

	fls := args.Filesystem

	//create the cloud data dir if necessary
	err := fls.MkdirAll(args.Config.CloudDataDir, 0700)
	if err != nil {
		return fmt.Errorf("failed to create the data directory %q: %w", args.Config.CloudDataDir, err)
	}

	//errW := args.ErrW
	config := args.Config
	outW := args.OutW
	dbPath := core.Path(config.AnonymousAccountDatabasePath)
	addr := "localhost:" + strconv.Itoa(args.Config.Port)
	host := core.Host("wss://" + addr)

	ctx, topCtx := createContext(host, args)
	defer topCtx.CancelGracefully()

	if args.RestrictProcessAccess {
		inoxprocess.RestrictProcessAccess(topCtx, inoxprocess.ProcessRestrictionConfig{
			ForceAllowDNS: true,
		})
	}

	accountDB, err := account.OpenAnonymousAccountDatabase(ctx, dbPath, fls)

	if err != nil {
		return err
	}
	fmt.Fprintf(outW, "anonymous account database opened (file %s)\n", dbPath)

	proxy := &cloudProxy{
		ctx:       ctx,
		topCtx:    topCtx,
		accountDB: accountDB,
		addr:      addr,
	}

	return proxy.run()
}

type cloudProxy struct {
	ctx, topCtx   *core.Context
	addr          string
	accountDB     *account.AnonymousAccountDatabase
	accountLogger zerolog.Logger
	proxyLogger   zerolog.Logger
}

func (p *cloudProxy) run() error {
	wsServer, err := net_ns.NewWebsocketServer(p.ctx)

	if err != nil {
		return err
	}

	p.accountLogger = p.ctx.Logger().With().Str(core.SOURCE_LOG_FIELD_NAME, ACCOUT_MANAGEMENT_LOG_SRC).Logger()
	p.proxyLogger = p.ctx.Logger().With().Str(core.SOURCE_LOG_FIELD_NAME, PROXY_LOG_SRC).Logger()

	//create a http server, register teardown callbacks and start listening.

	httpServer, err := http_ns.NewGolangHttpServer(p.ctx, http_ns.GolangHttpServerConfig{
		Addr: p.addr,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			if r.URL.Path == ACCOUNT_REGISTRATION_URL_PATH {
				hoster := r.URL.Query().Get("hoster")
				if hoster == "" {
					w.WriteHeader(http.StatusBadRequest)
					return
				}

				socket, err := wsServer.UpgradeGoValues(w, r, p.allowConnection)

				if err != nil {
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				go p.registerAccount(socket, hoster)
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

			account, err := p.accountDB.GetAccount(p.ctx, tokenHeaderValues[0])
			if err != nil {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}

			p.accountLogger.Info().Msg("account successfully connected: " + account.ULID)

			id := account.ULID
			_ = id
			//TODO: connect to account-specific project server.

			_, err = wsServer.UpgradeGoValues(w, r, p.allowConnection)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
			}
		}),
	})

	if err != nil {
		return err
	}

	p.ctx.OnGracefulTearDown(func(ctx *core.Context) error {
		return httpServer.Shutdown(ctx)
	})

	p.ctx.OnDone(func(timeoutCtx context.Context) error {
		return httpServer.Close()
	})

	p.proxyLogger.Info().Msgf("start cloud proxy HTTPS server listening on %s", p.addr)

	err = httpServer.ListenAndServeTLS("", "")
	if err != nil {
		return fmt.Errorf("failed to create HTTPS server for cloud proxy: %w", err)
	}

	return nil
}

func (p *cloudProxy) allowConnection(remoteAddrPort nettypes.RemoteAddrWithPort, remoteAddr nettypes.RemoteIpAddr, currentConns []*net_ns.WebsocketConnection) error {
	return nil
}

func (p *cloudProxy) registerAccount(socket *net_ns.WebsocketConnection, hoster string) {
	defer utils.Recover()
	defer socket.Close()

	//Bidirectionnial connection used to interactively register the account with the user.
	//CreateAnonymousAccountInteractively will show information to the user and the user will send several inputs.
	conn := &account.Connection{
		PrintFn: func(text string) {
			socket.WriteMessage(p.ctx, net_ns.WebsocketTextMessage, []byte(text))
		},
		ReadChan: make(chan string, 5),
	}

	receivedMsgChan := make(chan net_ns.WebsocketMessageChanItem, 10)
	err := socket.StartReadingAllMessagesIntoChan(p.ctx, receivedMsgChan)
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

	err = account.CreateAnonymousAccountInteractively(p.ctx, hoster, conn, p.accountDB)
	if err != nil {
		p.accountLogger.Err(err).Send()
	}
}
