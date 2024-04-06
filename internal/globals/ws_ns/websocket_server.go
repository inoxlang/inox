package ws_ns

import (
	"errors"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/permbase"
	"github.com/inoxlang/inox/internal/globals/http_ns"
	netaddr "github.com/inoxlang/inox/internal/netaddr"
	"github.com/inoxlang/inox/internal/utils"
)

const (
	DEFAULT_SERVER_HANDSHAKE_TIMEOUT = 3 * time.Second
	DEFAULT_SERVER_READ_BUFFER_SIZE  = 10_000
	DEFAULT_SERVER_WRITE_BUFFER_SIZE = 10_000
	DEFAULT_MAX_CONN_MSG_SIZE        = 100_000
	DEFAULT_MAX_IP_WS_CONNS          = 10

	//WEBSOCKET_CLOSE_TASK_PER_GOROUTINE  = 10
	SERVER_SIDE_WEBSOCKET_CLOSE_TIMEOUT = 2 * time.Second
	WEBSOCKET_SERVER_CLOSE_TIMEOUT      = 3 * time.Second

	DEFAULT_WRITE_MESSAGE_TIMEOUT         = 10 * time.Second
	DEFAULT_WAIT_FOR_NEXT_MESSAGE_TIMEOUT = 20 * time.Second
	DEFAULT_HANDSHAKE_TIMEOUT             = 5 * time.Second
)

var (
	ErrClosedWebsocketServer    = errors.New("closed websocket server")
	ErrTooManyWsConnectionsOnIp = errors.New("too many websocket connections on same ip")
)

// WebsocketServer upgrades an HTTP connection to a Websocket connection, it implements Value.
type WebsocketServer struct {
	upgrader        *websocket.Upgrader
	closingOrClosed atomic.Bool

	messageTimeout time.Duration

	connectionMapLock         sync.Mutex
	connections               map[netaddr.RemoteIpAddr]*[]*WebsocketConnection
	connectionsToClose        chan (*WebsocketConnection)
	closeMainClosingGoroutine chan (struct{})

	originalContext *core.Context
}

func NewWebsocketServer(ctx *core.Context) (*WebsocketServer, error) {
	return newWebsocketServer(ctx, DEFAULT_WRITE_MESSAGE_TIMEOUT)
}

func newWebsocketServer(ctx *core.Context, messageTimeout time.Duration) (*WebsocketServer, error) {

	perm := core.WebsocketPermission{Kind_: permbase.Provide}

	if err := ctx.CheckHasPermission(perm); err != nil {
		return nil, err
	}

	server := &WebsocketServer{
		connections:               map[netaddr.RemoteIpAddr]*[]*WebsocketConnection{},
		messageTimeout:            messageTimeout,
		connectionsToClose:        make(chan *WebsocketConnection, 100),
		closeMainClosingGoroutine: make(chan struct{}, 1),

		upgrader: &websocket.Upgrader{
			HandshakeTimeout: DEFAULT_SERVER_HANDSHAKE_TIMEOUT,
			ReadBufferSize:   DEFAULT_SERVER_READ_BUFFER_SIZE,
			WriteBufferSize:  DEFAULT_SERVER_WRITE_BUFFER_SIZE,
			//TODO: CheckOrigin: ,
			EnableCompression: true,
		},
		originalContext: ctx,
	}

	//spawn a goroutine to close connections.
	go func() {
		defer utils.Recover()

	loop:
		for {
			select {
			case conn := <-server.connectionsToClose:
				func() {
					defer utils.Recover()
					conn.closeNoCheck()
				}()

				server.removeConnection(conn)
			case <-server.closeMainClosingGoroutine:
				break loop
			}
		}
	}()

	return server, nil
}

func (s *WebsocketServer) Upgrade(rw *http_ns.ResponseWriter, r *http_ns.Request) (*WebsocketConnection, error) {
	conn, err := s.UpgradeGoValues(rw.DetachRespWriter(), r.Request(), nil)
	if err != nil {
		return nil, err
	}

	return conn, nil
}

// UpgradeGoValues first determines if the connection is allowed by calling allowConnectionFn,
// and then upgrades the HTTP server connection to the WebSocket protocol.
// If allowConnectionFn is nil the connection is accepted if the number of connections on the IP
// is less or equal to DEFAULT_MAX_IP_WS_CONNS.
// The execution of allowConnectionFn should be very quick because the server is locked during
// the UpgradeGoValues call.
func (s *WebsocketServer) UpgradeGoValues(
	rw http.ResponseWriter,
	r *http.Request,
	allowConnectionFn func(remoteAddrPort netaddr.RemoteAddrWithPort, remoteAddr netaddr.RemoteIpAddr, currentConns []*WebsocketConnection) error,
) (*WebsocketConnection, error) {

	if s.closingOrClosed.Load() {
		return nil, ErrClosedWebsocketServer
	}

	//limit number of concurrent connections per IP.

	s.connectionMapLock.Lock()
	defer s.connectionMapLock.Unlock()

	remoteAddrAndPort := netaddr.RemoteAddrWithPort(r.RemoteAddr)
	ip := remoteAddrAndPort.RemoteIp()

	conns := s.connections[ip]
	if conns == nil {
		conns = &[]*WebsocketConnection{}
		s.connections[ip] = conns
	}

	if allowConnectionFn != nil {
		err := allowConnectionFn(remoteAddrAndPort, ip, *conns)
		if err != nil {
			return nil, err
		}
	} else if len(*conns) >= DEFAULT_MAX_IP_WS_CONNS {
		return nil, ErrTooManyWsConnectionsOnIp
	}

	//create connection
	conn, err := s.upgrader.Upgrade(rw, r, nil)
	if err != nil {
		return nil, err
	}

	//configure connection
	conn.SetReadLimit(DEFAULT_MAX_CONN_MSG_SIZE)

	scheme := "ws"
	if r.URL.Scheme == "https" {
		scheme = "wss"
	}

	wsConn := &WebsocketConnection{
		conn:               conn,
		endpoint:           core.URL(r.URL.String()).WithScheme(core.Scheme(scheme)),
		remoteAddrWithPort: remoteAddrAndPort,
		writeTimeout:       s.messageTimeout,

		server:        s,
		serverContext: s.originalContext,
	}

	conn.SetPingHandler(func(message string) error {

		//Send pong.
		err := func() error {
			wsConn.writerLock.Lock()
			defer wsConn.writerLock.Unlock()

			return conn.WriteControl(websocket.PongMessage, []byte(message), time.Now().Add(s.messageTimeout))
		}()

		//Update the read deadline to keep the connection alive.
		func() {
			//If reading is true we know the ping handler is being called during a ReadMessage() or readJSON() call.
			//.readerLock is acquired by the goroutine calling the handler.
			if wsConn.reading.Load() {
				wsConn.setReadDeadlineNextMessageNoLock()
			}
		}()

		if err == websocket.ErrCloseSent {
			return nil
		} else if e, ok := err.(net.Error); ok && e.Temporary() {
			return nil
		}
		return err
	})

	*conns = append(*conns, wsConn)

	return wsConn, nil
}

func (s *WebsocketServer) removeConnection(conn *WebsocketConnection) {
	s.connectionMapLock.Lock()
	defer s.connectionMapLock.Unlock()

	sameIpConns, ok := s.connections[conn.remoteAddrWithPort.RemoteIp()]
	if ok {
		for index, c := range *sameIpConns {
			if c == conn {
				*sameIpConns = utils.RemoveIndexOfSlice(*sameIpConns, index)
				break
			}
		}
	}
}

func (s *WebsocketServer) Close(ctx *core.Context) error {
	if !s.closingOrClosed.CompareAndSwap(false, true) {
		return ErrClosedWebsocketServer
	}

	s.connectionMapLock.Lock()
	connections := s.connections
	s.connections = nil
	s.connectionMapLock.Unlock()

	//add connections to the connectionsToClose channel.
	for _, conns := range connections {
		if conns == nil {
			continue
		}
		for _, conn := range *conns {
			if !conn.closingOrClosed.Load() {
				s.connectionsToClose <- conn
			}
		}
	}

	remainingTime := WEBSOCKET_SERVER_CLOSE_TIMEOUT
	deadline := time.Now().Add(remainingTime)

loop:
	for {
		select {
		case conn := <-s.connectionsToClose:
			func() {
				defer utils.Recover()
				conn.closeNoCheck()
			}()

			s.removeConnection(conn)
			remainingTime = time.Until(deadline)
		case <-time.After(remainingTime):
			break loop
		}
	}
	s.closeMainClosingGoroutine <- struct{}{}
	//help the closing goroutine.
	return nil
}
