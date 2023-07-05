package lsp

import (
	"fmt"
	"net/http"

	core "github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/net_ns"
	"github.com/inoxlang/inox/internal/project_server/jsonrpc"
	"github.com/rs/zerolog"
)

const (
	JSON_RPC_SERVER_LOGC_SRC = "/json-rpc"
)

type JsonRpcWebsocketServer struct {
	wsServer  *net_ns.WebsocketServer
	rpcServer *jsonrpc.Server
	logger    *zerolog.Logger
}

type JsonRpcWebsocketServerConfig struct {
	addr      string
	rpcServer *jsonrpc.Server
}

func NewJsonRpcWebsocketServer(ctx *core.Context, config JsonRpcWebsocketServerConfig) (*JsonRpcWebsocketServer, error) {

	logger := *ctx.Logger()
	logger = logger.With().Str(core.SOURCE_LOG_FIELD_NAME, JSON_RPC_SERVER_LOGC_SRC).Logger()

	wsServer, err := net_ns.NewWebsocketServer(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create websocket server: %w", err)
	}

	server := &JsonRpcWebsocketServer{
		wsServer:  wsServer,
		logger:    &logger,
		rpcServer: config.rpcServer,
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create HTTPS server: %w", err)
	}

	return server, nil
}

func (server *JsonRpcWebsocketServer) handleNew(httpRespWriter http.ResponseWriter, httpReq *http.Request) {
	conn, err := server.wsServer.UpgradeGoValues(httpRespWriter, httpReq)
	if err != nil {
		server.logger.Debug().Err(err).Send()
		return
	}

	socket := NewJsonRpcWebsocket(conn, *server.logger)
	server.rpcServer.MsgConnComeIn(socket, func(session *jsonrpc.Session) {
		socket.sessionContext = session.Context()
	})
}
