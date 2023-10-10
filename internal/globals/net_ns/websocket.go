package net_ns

import (
	"errors"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"

	core "github.com/inoxlang/inox/internal/core"
	nettypes "github.com/inoxlang/inox/internal/net_types"
	"github.com/inoxlang/inox/internal/permkind"
)

var (
	ErrClosingOrClosedWebsocketConn = errors.New("closed or closing websocket connection")
)

type WebsocketMessageType int

const (
	WebsocketBinaryMessage WebsocketMessageType = websocket.BinaryMessage
	WebsocketTextMessage   WebsocketMessageType = websocket.TextMessage
	WebsocketPingMessage   WebsocketMessageType = websocket.PingMessage
	WebsocketPongMessage   WebsocketMessageType = websocket.PongMessage
	WebsocketCloseMessage  WebsocketMessageType = websocket.CloseMessage
)

type WebsocketConnection struct {
	conn               *websocket.Conn
	remoteAddrWithPort nettypes.RemoteAddrWithPort
	endpoint           core.URL //HTTP endpoint

	messageTimeout  time.Duration
	closingOrClosed atomic.Bool
	tokenGivenBack  atomic.Bool

	server *WebsocketServer //nil on client side

	serverContext *core.Context
}

func (conn *WebsocketConnection) RemoteAddrWithPort() nettypes.RemoteAddrWithPort {
	return conn.remoteAddrWithPort
}

func (conn *WebsocketConnection) GetGoMethod(name string) (*core.GoFunction, bool) {
	switch name {
	case "sendJSON":
		return core.WrapGoMethod(conn.sendJSON), true
	case "readJSON":
		return core.WrapGoMethod(conn.readJSON), true
	case "close":
		return core.WrapGoMethod(conn.Close), true
	}
	return nil, false
}

func (conn *WebsocketConnection) Prop(ctx *core.Context, name string) core.Value {
	method, ok := conn.GetGoMethod(name)
	if !ok {
		panic(core.FormatErrPropertyDoesNotExist(name, conn))
	}
	return method
}

func (*WebsocketConnection) SetProp(ctx *core.Context, name string, value core.Value) error {
	return core.ErrCannotSetProp
}

func (*WebsocketConnection) PropertyNames(ctx *core.Context) []string {
	return []string{"sendJSON", "readJSON", "close"}
}

func (conn *WebsocketConnection) SetPingHandler(ctx *core.Context, handler func(data string) error) {
	conn.conn.SetPingHandler(handler)
}

func (conn *WebsocketConnection) sendJSON(ctx *core.Context, msg core.Value) error {
	if err := conn.checkWriteAndConfig(ctx); err != nil {
		return err
	}

	err := conn.conn.WriteJSON(core.ToJSONVal(ctx, msg.(core.Serializable)))
	conn.conn.SetWriteDeadline(time.Now().Add(DEFAULT_WS_WAIT_MESSAGE_TIMEOUT))
	conn.closeIfNecessary(err)

	return err
}

func (conn *WebsocketConnection) readJSON(ctx *core.Context) (core.Value, error) {
	if err := conn.checkReadAndConfig(ctx); err != nil {
		return nil, err
	}

	var v interface{}
	err := conn.conn.ReadJSON(&v)
	conn.conn.SetReadDeadline(time.Now().Add(DEFAULT_WS_WAIT_MESSAGE_TIMEOUT))

	conn.closeIfNecessary(err)

	if err != nil {
		return nil, err
	}

	return core.ConvertJSONValToInoxVal(v, false), nil
}

func (conn *WebsocketConnection) ReadMessage(ctx *core.Context) (messageType WebsocketMessageType, p []byte, err error) {
	if err := conn.checkReadAndConfig(ctx); err != nil {
		return 0, nil, err
	}

	msgType, p, err := conn.conn.ReadMessage()
	conn.conn.SetReadDeadline(time.Now().Add(DEFAULT_WS_WAIT_MESSAGE_TIMEOUT))
	conn.closeIfNecessary(err)

	return WebsocketMessageType(msgType), p, err
}

func (conn *WebsocketConnection) WriteMessage(ctx *core.Context, messageType WebsocketMessageType, data []byte) error {
	if err := conn.checkWriteAndConfig(ctx); err != nil {
		return err
	}

	err := conn.conn.WriteMessage(int(messageType), data)
	conn.conn.SetReadDeadline(time.Now().Add(DEFAULT_WS_WAIT_MESSAGE_TIMEOUT))
	conn.closeIfNecessary(err)

	return err
}

func (conn *WebsocketConnection) checkReadAndConfig(ctx *core.Context) error {
	if conn.closingOrClosed.Load() {
		return ErrClosingOrClosedWebsocketConn
	}

	//if on client side
	if conn.server == nil {
		perm := core.WebsocketPermission{
			Kind_:    permkind.Read,
			Endpoint: conn.endpoint,
		}

		if err := ctx.CheckHasPermission(perm); err != nil {
			return err
		}
	}

	conn.conn.SetReadDeadline(time.Now().Add(conn.messageTimeout))
	return nil
}

func (conn *WebsocketConnection) checkWriteAndConfig(ctx *core.Context) error {
	if conn.closingOrClosed.Load() {
		return ErrClosingOrClosedWebsocketConn
	}

	//if on client side
	if conn.server == nil {
		perm := core.WebsocketPermission{
			Kind_:    permkind.WriteStream,
			Endpoint: conn.endpoint,
		}

		if err := ctx.CheckHasPermission(perm); err != nil {
			return err
		}
	}

	conn.conn.SetWriteDeadline(time.Now().Add(conn.messageTimeout))
	return nil
}

func (conn *WebsocketConnection) closeIfNecessary(err error) {
	if closeErr, ok := err.(*websocket.CloseError); ok {
		switch closeErr.Code {
		case websocket.CloseNormalClosure,
			websocket.CloseGoingAway,
			websocket.CloseNoStatusReceived:
			conn.Close()
			return
		}
	}

	if err != nil && strings.Contains(err.Error(), "i/o timeout") {
		conn.Close()
	}
}

func (conn *WebsocketConnection) IsClosedOrClosing() bool {
	return conn.closingOrClosed.Load()
}

func (conn *WebsocketConnection) Close() error {
	if !conn.closingOrClosed.CompareAndSwap(false, true) {
		return ErrClosingOrClosedWebsocketConn
	}

	//server side websockets are managed by the server.
	if conn.server != nil {
		conn.server.connectionsToClose <- conn
		return nil
	}

	return conn.closeNoCheck()
}

func (conn *WebsocketConnection) closeNoCheck() error {
	conn.conn.WriteControl(int(WebsocketCloseMessage), nil, time.Now().Add(SERVER_SIDE_WEBSOCKET_CLOSE_TIMEOUT))

	if conn.tokenGivenBack.CompareAndSwap(false, true) {
		conn.serverContext.GiveBack(WS_SIMUL_CONN_TOTAL_LIMIT_NAME, 1)
	}

	return conn.conn.Close()
}
