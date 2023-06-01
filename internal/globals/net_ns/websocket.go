package net_ns

import (
	"errors"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	core "github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/http_ns"
	"github.com/inoxlang/inox/internal/permkind"
)

var ErrClosedWebsocketConnection = errors.New("closed websocket connection")

type WebsocketMessageType int

const (
	WebsocketBinaryMessage WebsocketMessageType = websocket.BinaryMessage
	WebsocketTextMessage                        = websocket.TextMessage
	WebsocketPingMessage                        = websocket.PingMessage
	WebsocketPongMessage                        = websocket.PongMessage
	WebsocketCloseMessage                       = websocket.CloseMessage
)

type WebsocketConnection struct {
	conn               *websocket.Conn
	remoteAddrWithPort http_ns.RemoteAddrWithPort
	endpoint           URL //HTTP endpoint

	messageTimeout time.Duration

	//prevent giving back tokens, this value should NOT be used to check if the connection is closed.
	closed atomic.Bool

	server *WebsocketServer //nil on client side

	originalContext *Context

	core.NotClonableMixin
	core.NoReprMixin
}

func (conn *WebsocketConnection) GetGoMethod(name string) (*GoFunction, bool) {
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

func (conn *WebsocketConnection) Prop(ctx *core.Context, name string) Value {
	method, ok := conn.GetGoMethod(name)
	if !ok {
		panic(core.FormatErrPropertyDoesNotExist(name, conn))
	}
	return method
}

func (*WebsocketConnection) SetProp(ctx *core.Context, name string, value core.Value) error {
	return core.ErrCannotSetProp
}

func (*WebsocketConnection) PropertyNames(ctx *Context) []string {
	return []string{"sendJSON", "readJSON", "close"}
}

func (conn *WebsocketConnection) sendJSON(ctx *Context, msg Value) error {
	if err := conn.checkWriteAndConfig(ctx); err != nil {
		return err
	}

	err := conn.conn.WriteJSON(core.ToJSONVal(ctx, msg))
	conn.closeIfNecessary(err)

	return err
}

func (conn *WebsocketConnection) readJSON(ctx *Context) (Value, error) {
	if err := conn.checkReadAndConfig(ctx); err != nil {
		return nil, err
	}

	var v interface{}
	err := conn.conn.ReadJSON(&v)

	conn.closeIfNecessary(err)

	if err != nil {
		return nil, err
	}

	return core.ConvertJSONValToInoxVal(ctx, v, false), nil
}

func (conn *WebsocketConnection) ReadMessage(ctx *Context) (messageType WebsocketMessageType, p []byte, err error) {
	if err := conn.checkReadAndConfig(ctx); err != nil {
		return 0, nil, err
	}

	msgType, p, err := conn.conn.ReadMessage()
	conn.closeIfNecessary(err)

	return WebsocketMessageType(msgType), p, err
}

func (conn *WebsocketConnection) WriteMessage(ctx *Context, messageType WebsocketMessageType, data []byte) error {
	if err := conn.checkWriteAndConfig(ctx); err != nil {
		return err
	}

	err := conn.conn.WriteMessage(int(messageType), data)
	conn.closeIfNecessary(err)

	return err
}

func (conn *WebsocketConnection) checkReadAndConfig(ctx *core.Context) error {
	if conn.closed.Load() {
		return ErrClosedWebsocketConnection
	}

	//if on client side
	if conn.server == nil {
		perm := WebsocketPermission{
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
	if conn.closed.Load() {
		return ErrClosedWebsocketConnection
	}

	//if on client side
	if conn.server == nil {
		perm := WebsocketPermission{
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

func (conn *WebsocketConnection) Close() error {
	if !conn.closed.CompareAndSwap(false, true) {
		return ErrClosedWebsocketConnection
	}

	if conn.server != nil && !conn.server.closingOrClosed.Load() {
		conn.server.removeConnection(conn)
	}

	conn.conn.WriteControl(WebsocketCloseMessage, nil, time.Now().Add(SERVER_SIDE_WEBSOCKET_CLOSE_TIMEOUT))

	conn.originalContext.GiveBack(WS_SIMUL_CONN_TOTAL_LIMIT_NAME, 1)
	return conn.conn.Close()
}
