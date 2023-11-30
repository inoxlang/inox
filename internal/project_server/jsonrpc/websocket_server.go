package jsonrpc

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/net_ns"
	nettypes "github.com/inoxlang/inox/internal/net_types"
	"github.com/inoxlang/inox/internal/project_server/logs"
	"github.com/rs/zerolog"
)

const (
	JSON_RPC_SERVER_LOG_SRC                 = "json-rpc"
	DEFAULT_MAX_IP_WS_CONNS                 = 3
	DEFAULT_MAX_IP_WS_CONNS_IF_BEHIND_PROXY = 10_000
)

var (
	ErrOnly127001AllowedIfBehindProxy = errors.New("only connections from the same host (127.0.0.1) are allowed")
)

type JsonRpcWebsocketServer struct {
	wsServer  *net_ns.WebsocketServer
	rpcServer *Server
	logger    *zerolog.Logger

	config JsonRpcWebsocketServerConfig
}

type JsonRpcWebsocketServerConfig struct {
	Addr      string
	RpcServer *Server

	//defaults to DEFAULT_MAX_IP_WS_CONNS
	MaxWebsocketPerIp int

	//if true only connections from localhost are allowed and
	//the effective value of MaxWebsocketPerIp is set to DEFAULT_MAX_IP_WS_CONNS_IF_BEHIND_PROXY.
	BehindCloudProxy bool
}

func NewJsonRpcWebsocketServer(ctx *core.Context, config JsonRpcWebsocketServerConfig) (*JsonRpcWebsocketServer, error) {

	logger := *ctx.Logger()
	logger = core.ChildLoggerWithSource(logger, JSON_RPC_SERVER_LOG_SRC)

	wsServer, err := net_ns.NewWebsocketServer(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create websocket server: %w", err)
	}

	if config.MaxWebsocketPerIp <= 0 {
		config.MaxWebsocketPerIp = DEFAULT_MAX_IP_WS_CONNS
	}

	if config.BehindCloudProxy {
		config.MaxWebsocketPerIp = DEFAULT_MAX_IP_WS_CONNS_IF_BEHIND_PROXY
	}

	server := &JsonRpcWebsocketServer{
		wsServer:  wsServer,
		logger:    &logger,
		rpcServer: config.RpcServer,
		config:    config,
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create HTTPS server: %w", err)
	}

	return server, nil
}

func (server *JsonRpcWebsocketServer) Logger() *zerolog.Logger {
	return server.logger
}

func (server *JsonRpcWebsocketServer) HandleNew(httpRespWriter http.ResponseWriter, httpReq *http.Request) {
	conn, err := server.wsServer.UpgradeGoValues(httpRespWriter, httpReq, server.allowNewConnection)
	if err != nil {
		server.logger.Debug().Err(err).Send()
		return
	}

	socket := NewJsonRpcWebsocket(conn, *server.logger)
	server.rpcServer.MsgConnComeIn(socket, func(session *Session) {
		logs.Printf("new session at %s (remote)\n", socket.conn.RemoteAddrWithPort())
		socket.sessionContext = session.Context()
	})
}

func (server *JsonRpcWebsocketServer) allowNewConnection(
	remoteAddrPort nettypes.RemoteAddrWithPort,
	remoteAddr nettypes.RemoteIpAddr,
	currentConns []*net_ns.WebsocketConnection) error {

	if server.config.BehindCloudProxy {
		if remoteAddr != "127.0.0.1" {
			return ErrOnly127001AllowedIfBehindProxy
		}
	}

	if len(currentConns)+1 <= server.config.MaxWebsocketPerIp {
		return nil
	}
	return net_ns.ErrTooManyWsConnectionsOnIp
}
