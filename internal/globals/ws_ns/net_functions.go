package ws_ns

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/websocket"

	"github.com/inoxlang/inox/internal/commonfmt"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/permkind"
	"github.com/inoxlang/inox/internal/utils"
)

const (
	WS_SIMUL_CONN_TOTAL_LIMIT_NAME = "ws/simul-connections"
	OPTION_DOES_NOT_EXIST_FMT      = "option '%s' does not exist"
)

func websocketConnect(ctx *core.Context, u core.URL, options ...core.Option) (*WebsocketConnection, error) {
	insecure := false

	for _, opt := range options {
		switch opt.Name {
		case "insecure":
			insecure = bool(opt.Value.(core.Bool))
		default:
			return nil, commonfmt.FmtErrInvalidOptionName(opt.Name)
		}
	}

	return WebsocketConnect(WebsocketConnectParams{
		Ctx:      ctx,
		URL:      u,
		Insecure: insecure,
	})
}

type WebsocketConnectParams struct {
	Ctx                 *core.Context
	URL                 core.URL
	Insecure            bool
	RequestHeader       http.Header
	WriteMessageTimeout time.Duration //if 0 defaults to DEFAULT_WRITE_MESSAGE_TIMEOUT
	HandshakeTimeout    time.Duration //if 0 defaults to DEFAULT_HANDSHAKE_TIMEOUT
}

func WebsocketConnect(args WebsocketConnectParams) (*WebsocketConnection, error) {
	ctx := args.Ctx
	u := args.URL
	insecure := args.Insecure
	requestHeader := args.RequestHeader
	writeMessageTimeout := utils.DefaultIfZero(args.WriteMessageTimeout, DEFAULT_WRITE_MESSAGE_TIMEOUT)
	handshakeTimeout := utils.DefaultIfZero(args.HandshakeTimeout, DEFAULT_HANDSHAKE_TIMEOUT)

	//check that a websocket read or write-stream permission is granted
	perm := core.WebsocketPermission{
		Kind_:    permkind.WriteStream,
		Endpoint: u,
	}

	if err := ctx.CheckHasPermission(perm); err != nil {
		perm.Kind_ = permkind.Read

		if err := ctx.CheckHasPermission(perm); err != nil {
			return nil, err
		}
	}

	ctx.Take(WS_SIMUL_CONN_TOTAL_LIMIT_NAME, 1)

	dialer := *websocket.DefaultDialer
	dialer.HandshakeTimeout = handshakeTimeout
	dialer.TLSClientConfig = &tls.Config{
		InsecureSkipVerify: insecure,
	}

	c, resp, err := dialer.Dial(string(u), requestHeader)
	if err != nil {
		ctx.GiveBack(WS_SIMUL_CONN_TOTAL_LIMIT_NAME, 1)

		if resp == nil {
			return nil, fmt.Errorf("dial: %s", err.Error())
		} else {
			return nil, fmt.Errorf("dial: %s (http status code: %d, text: %s)", err.Error(), resp.StatusCode, resp.Status)
		}
	}

	return &WebsocketConnection{
		conn:          c,
		endpoint:      u,
		writeTimeout:  writeMessageTimeout,
		serverContext: ctx,
	}, nil
}
