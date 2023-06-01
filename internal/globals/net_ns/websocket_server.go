package net_ns

import (
	"errors"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	core "github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/http_ns"
	"github.com/inoxlang/inox/internal/permkind"
	"github.com/inoxlang/inox/internal/utils"
)

const (
	DEFAULT_WS_SERVER_HANDSHAKE_TIMEOUT = 3 * time.Second
	DEFAULT_WS_SERVER_READ_BUFFER_SIZE  = 10_000
	DEFAULT_WS_SERVER_WRITE_BUFFER_SIZE = 10_000
	DEFAULT_MAX_WS_CONN_MSG_SIZE        = 100_000
	DEFAULT_MAX_IP_WS_CONNS             = 10

	WEBSOCKET_CLOSE_TASK_PER_GOROUTINE  = 10
	SERVER_SIDE_WEBSOCKET_CLOSE_TIMEOUT = 2 * time.Second
)

var (
	ErrClosedWebsocketServer    = errors.New("closed websocket server")
	ErrTooManyWsConnectionsOnIp = errors.New("too many websocket connections on same ip")
)

// WebsocketServer is a LSP server that uses Websocket to exchange messages with the client.
type WebsocketServer struct {
	core.NotClonableMixin
	core.NoReprMixin
	upgrader        *websocket.Upgrader
	closingOrClosed atomic.Bool

	messageTimeout time.Duration

	connectionMapLock sync.Mutex
	connections       map[http_ns.RemoteIpAddr]*[]*WebsocketConnection

	originalContext *Context
}

func NewWebsocketServer(ctx *Context) (*WebsocketServer, error) {
	return newWebsocketServer(ctx, DEFAULT_WS_TIMEOUT)
}

func newWebsocketServer(ctx *Context, messageTimeout time.Duration) (*WebsocketServer, error) {

	perm := WebsocketPermission{Kind_: permkind.Provide}

	if err := ctx.CheckHasPermission(perm); err != nil {
		return nil, err
	}

	return &WebsocketServer{
		connections:    map[http_ns.RemoteIpAddr]*[]*WebsocketConnection{},
		messageTimeout: messageTimeout,

		upgrader: &websocket.Upgrader{
			HandshakeTimeout: DEFAULT_WS_SERVER_HANDSHAKE_TIMEOUT,
			ReadBufferSize:   DEFAULT_WS_SERVER_READ_BUFFER_SIZE,
			WriteBufferSize:  DEFAULT_WS_SERVER_WRITE_BUFFER_SIZE,
			//TODO: CheckOrigin: ,
			EnableCompression: true,
		},
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

func (s *WebsocketServer) Upgrade(rw *http_ns.HttpResponseWriter, r *http_ns.HttpRequest) (*WebsocketConnection, error) {
	conn, err := s.UpgradeGoValues(rw.RespWriter(), r.Request())
	if err != nil {
		return nil, err
	}

	return conn, nil
}

func (s *WebsocketServer) UpgradeGoValues(rw http.ResponseWriter, r *http.Request) (*WebsocketConnection, error) {
	if s.closingOrClosed.Load() {
		return nil, ErrClosedWebsocketServer
	}

	//limit number of concurrent connections per IP.

	s.connectionMapLock.Lock()
	defer s.connectionMapLock.Unlock()

	remoteAddrAndPort := http_ns.RemoteAddrWithPort(r.RemoteAddr)
	ip := remoteAddrAndPort.RemoteIp()

	conns := s.connections[ip]
	if conns == nil {
		conns = &[]*WebsocketConnection{}
		s.connections[ip] = conns
	} else if len(*conns) >= DEFAULT_MAX_IP_WS_CONNS {
		return nil, ErrTooManyWsConnectionsOnIp
	}

	//create connection
	conn, err := s.upgrader.Upgrade(rw, r, nil)
	if err != nil {
		return nil, err
	}

	//configure connection
	conn.SetReadLimit(DEFAULT_MAX_WS_CONN_MSG_SIZE)

	scheme := "ws"
	if r.URL.Scheme == "https" {
		scheme = "wss"
	}

	wsConn := &WebsocketConnection{
		conn:               conn,
		endpoint:           core.URL(r.URL.String()).WithScheme(core.Scheme(scheme)),
		remoteAddrWithPort: remoteAddrAndPort,
		messageTimeout:     s.messageTimeout,

		server:          s,
		originalContext: s.originalContext,
	}

	*conns = append(*conns, wsConn)

	return wsConn, nil
}

func (s *WebsocketServer) removeConnection(conn *WebsocketConnection) {
	s.connectionMapLock.Lock()
	defer s.connectionMapLock.Unlock()

	s.removeConnectionNoLock(conn)
}

func (s *WebsocketServer) removeConnectionNoLock(conn *WebsocketConnection) {
	ip := conn.remoteAddrWithPort.RemoteIp()

	conns := s.connections[ip]
	if conns == nil {
		return
	}

	index := -1
	for i, c := range *conns {
		if c.conn == conn.conn {
			index = i
			break
		}
	}

	if index >= 0 {
		*conns = utils.RemoveIndexOfSlice(*conns, index)
	}
}

func (s *WebsocketServer) Close(ctx *Context) error {
	if !s.closingOrClosed.CompareAndSwap(false, true) {
		return ErrClosedWebsocketServer
	}

	s.connectionMapLock.Lock()
	defer s.connectionMapLock.Unlock()

	var allConnections []*WebsocketConnection

	for ip, conns := range s.connections {
		allConnections = append(allConnections, (*conns)...)
		delete(s.connections, ip)
	}

	wg := new(sync.WaitGroup)
	goroutineCount := (len(allConnections) / WEBSOCKET_CLOSE_TASK_PER_GOROUTINE) + (len(allConnections) % WEBSOCKET_CLOSE_TASK_PER_GOROUTINE)
	wg.Add(goroutineCount)

	//we create a goroutine for each group of CLOSE_TASK_PER_GOROUTINE connections.
	for i := 0; i < goroutineCount; i++ {
		startIndex := i * WEBSOCKET_CLOSE_TASK_PER_GOROUTINE
		endIndex := utils.Min(len(allConnections), (i+1)*WEBSOCKET_CLOSE_TASK_PER_GOROUTINE)
		if endIndex > startIndex {
			break
		}

		go func(conns []*WebsocketConnection) {
			defer wg.Done()

			for _, conn := range conns {
				func(conn *WebsocketConnection) {
					defer recover()
					conn.Close()
				}(conn)
			}
		}(allConnections[startIndex:endIndex])
	}

	wg.Wait()

	for ip, conns := range s.connections {
		allConnections = append(allConnections, (*conns)...)
		delete(s.connections, ip)
	}

	return nil
}
