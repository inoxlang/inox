package ws_ns

import (
	"errors"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/permkind"
	ws_symbolic "github.com/inoxlang/inox/internal/globals/ws_ns/symbolic"
	netaddr "github.com/inoxlang/inox/internal/netaddr"
	"github.com/inoxlang/inox/internal/utils"
)

var (
	ErrClosingOrClosedWebsocketConn = errors.New("closed or closing websocket connection")
	ErrAlreadyReadingAllMessages    = errors.New("already reading all messages")
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
	remoteAddrWithPort netaddr.RemoteAddrWithPort
	endpoint           core.URL //HTTP endpoint

	messageTimeout               time.Duration
	closingOrClosed              atomic.Bool
	tokenGivenBack               atomic.Bool
	isReadingAllMessagesIntoChan atomic.Bool

	server *WebsocketServer //nil on client side

	serverContext *core.Context
}

func (conn *WebsocketConnection) RemoteAddrWithPort() netaddr.RemoteAddrWithPort {
	return conn.remoteAddrWithPort
}

func (conn *WebsocketConnection) GetGoMethod(name string) (*core.GoFunction, bool) {
	switch name {
	case "send_json":
		return core.WrapGoMethod(conn.sendJSON), true
	case "read_json":
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
	return ws_symbolic.WEBSOCKET_PROPNAMES
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

	//TODO: use ParseJSONRepresentation (add tests before change)
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

type WebsocketMessageChanItem struct {
	Error   error
	Type    WebsocketMessageType //nil if error
	Payload []byte               //nil if error
}

// StartReadingAllMessagesIntoChan creates a goroutine that continuously calls ReadMessage() and puts results in channel.
// The goroutine stops when the context is done or the connection is closed or closing; the channel is closed.
// If the connection is already reading all messages ErrAlreadyReadingAllMessages is returned and the channel is not closed.
// If the connection is connection is closed ErrClosingOrClosedWebsocketConn is returned and the channel is not closed.
func (conn *WebsocketConnection) StartReadingAllMessagesIntoChan(ctx *core.Context, channel chan WebsocketMessageChanItem) error {

	if !conn.isReadingAllMessagesIntoChan.CompareAndSwap(false, true) {
		return ErrAlreadyReadingAllMessages
	}

	if conn.IsClosedOrClosing() {
		return ErrClosingOrClosedWebsocketConn
	}

	go func() {
		defer utils.Recover()
		defer conn.isReadingAllMessagesIntoChan.Store(true)
		defer close(channel)

		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			msgType, payload, err := conn.ReadMessage(ctx)
			if conn.closingOrClosed.Load() {
				if err != nil {
					channel <- WebsocketMessageChanItem{
						Error: err,
					}
				}
				channel <- WebsocketMessageChanItem{
					Error: ErrClosingOrClosedWebsocketConn,
				}
				return
			}

			if err != nil {
				channel <- WebsocketMessageChanItem{
					Error: err,
				}
			} else {
				channel <- WebsocketMessageChanItem{
					Type:    msgType,
					Payload: payload,
				}
			}
		}
	}()

	return nil
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

	//TODO: find out why the deadline cannot be overriden here after a call to WriteMessage invoked SetReadDeadline.
	//setting a custom timeout here has no effect, why ?
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
