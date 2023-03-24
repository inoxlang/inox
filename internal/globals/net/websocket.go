package internal

import (
	"errors"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	core "github.com/inox-project/inox/internal/core"
)

var ErrClosedWebsocketConnection = errors.New("closed websocket connection")

type WebsocketConnection struct {
	core.NotClonableMixin
	core.NoReprMixin
	conn            *websocket.Conn
	endpoint        URL   //HTTP endpoint
	closed          int32 //prevent giving back tokens
	originalContext *Context
}

func (conn *WebsocketConnection) GetGoMethod(name string) (*GoFunction, bool) {
	switch name {
	case "sendJSON":
		return core.WrapGoMethod(conn.sendJSON), true
	case "readJSON":
		return core.WrapGoMethod(conn.readJSON), true
	case "close":
		return core.WrapGoMethod(conn.close), true
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
	if atomic.LoadInt32(&conn.closed) != 0 {
		return ErrClosedWebsocketConnection
	}

	perm := WebsocketPermission{
		Kind_:    core.WritePerm,
		Endpoint: conn.endpoint,
	}

	if err := ctx.CheckHasPermission(perm); err != nil {
		return err
	}

	conn.conn.SetWriteDeadline(time.Now().Add(DEFAULT_WS_TIMEOUT))
	err := conn.conn.WriteJSON(core.ToJSONVal(ctx, msg))

	// only way to check a websocket.netError
	if err != nil && strings.Contains(err.Error(), "i/o timeout") {
		conn.close(ctx)
	}

	return err
}

func (conn *WebsocketConnection) readJSON(ctx *Context) (Value, error) {
	if atomic.LoadInt32(&conn.closed) != 0 {
		return nil, ErrClosedWebsocketConnection
	}

	perm := WebsocketPermission{
		Kind_:    core.ReadPerm,
		Endpoint: conn.endpoint,
	}

	if err := ctx.CheckHasPermission(perm); err != nil {
		return nil, err
	}

	var v interface{}
	conn.conn.SetReadDeadline(time.Now().Add(DEFAULT_WS_TIMEOUT))
	err := conn.conn.ReadJSON(&v)

	// only way to check a websocket.netError
	if err != nil && strings.Contains(err.Error(), "i/o timeout") {
		conn.close(ctx)
	}

	if err != nil {
		return nil, err
	}

	return core.ConvertJSONValToInoxVal(ctx, v, false), nil
}

func (conn *WebsocketConnection) close(ctx *Context) error {
	if !atomic.CompareAndSwapInt32(&conn.closed, 0, 1) {
		return ErrClosedWebsocketConnection
	}

	conn.originalContext.GiveBack(WS_SIMUL_CONN_TOTAL_LIMIT_NAME, 1)
	return conn.conn.Close()
}
