package internal

import (
	"errors"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	core "github.com/inox-project/inox/internal/core"
	_http "github.com/inox-project/inox/internal/globals/http"
)

const (
	DEFAULT_WS_SERVER_HANDSHAKE_TIMEOUT = 3 * time.Second
	DEFAULT_WS_SERVER_READ_BUFFER_SIZE  = 4_000
	DEFAULT_WS_SERVER_WRITE_BUFFER_SIZE = 4_000
)

var ErrClosedWebsocketServer = errors.New("closed websocket server")

type WebsocketServer struct {
	core.NotClonableMixin
	core.NoReprMixin
	upgrader        *websocket.Upgrader
	closed          int32
	originalContext *Context
}

func NewWebsocketServer(ctx *Context) (*WebsocketServer, error) {

	perm := WebsocketPermission{Kind_: core.ProvidePerm}

	if err := ctx.CheckHasPermission(perm); err != nil {
		return nil, err
	}

	return &WebsocketServer{
		upgrader: &websocket.Upgrader{
			HandshakeTimeout: DEFAULT_WS_SERVER_HANDSHAKE_TIMEOUT,
			ReadBufferSize:   DEFAULT_WS_SERVER_READ_BUFFER_SIZE,
			WriteBufferSize:  DEFAULT_WS_SERVER_WRITE_BUFFER_SIZE,
			//TODO: WriteBufferPool: ,
			//TODO: CheckOrigin: ,
			EnableCompression: true,
		},
		closed:          0,
		originalContext: ctx,
	}, nil
}

func (s *WebsocketServer) GetGoMethod(name string) (*GoFunction, bool) {
	switch name {
	case "upgrade":
		return core.WrapGoMethod(s.Upgrade), true
	case "close":
		return core.WrapGoMethod(s.Close), true
	}
	return nil, false
}

func (s *WebsocketServer) Prop(ctx *core.Context, name string) Value {
	method, ok := s.GetGoMethod(name)
	if !ok {
		panic(core.FormatErrPropertyDoesNotExist(name, s))
	}
	return method
}

func (*WebsocketServer) SetProp(ctx *core.Context, name string, value core.Value) error {
	return core.ErrCannotSetProp
}

func (*WebsocketServer) PropertyNames(ctx *Context) []string {
	return []string{"upgrade", "close"}
}

func (s *WebsocketServer) Upgrade(rw *_http.HttpResponseWriter, r *_http.HttpRequest) (*WebsocketConnection, error) {
	if atomic.LoadInt32(&s.closed) != 0 {
		return nil, ErrClosedWebsocketServer
	}
	conn, err := s.upgrader.Upgrade(rw.RespWriter(), r.Request(), nil)
	if err != nil {
		return nil, err
	}

	//TODO: limiter number of concurrent connections for a given IP

	scheme := "ws"
	if r.URL.Scheme() == "https" {
		scheme = "wss"
	}

	return &WebsocketConnection{
		conn:            conn,
		endpoint:        r.URL.WithScheme(core.Scheme(scheme)),
		closed:          0,
		originalContext: s.originalContext,
	}, nil
}

func (s *WebsocketServer) Close(ctx *Context) error {
	if !atomic.CompareAndSwapInt32(&s.closed, 0, 1) {
		return ErrClosedWebsocketServer
	}
	return nil
}
