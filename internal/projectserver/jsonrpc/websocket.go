package jsonrpc

import (
	"strconv"
	"sync"

	"github.com/gorilla/websocket"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/ws_ns"
	"github.com/rs/zerolog"
)

const (
	CONTENT_LENGTH_HEADER = "Content-Length: "
)

var (
	_ MessageReaderWriter = (*JsonRpcWebsocket)(nil)
)

type JsonRpcWebsocket struct {
	conn           *ws_ns.WebsocketConnection
	lock           sync.RWMutex
	logger         zerolog.Logger
	sessionContext *core.Context //set after session is created.
}

func NewJsonRpcWebsocket(conn *ws_ns.WebsocketConnection, logger zerolog.Logger) *JsonRpcWebsocket {
	return &JsonRpcWebsocket{conn: conn}
}

func (s *JsonRpcWebsocket) ReadMessage() ([]byte, error) {
	msgType, msg, err := s.conn.ReadMessage(s.sessionContext)
	if err != nil {
		s.logger.Err(err).Msg("error while reading message from websocket")
		return nil, err
	}

	if msgType != websocket.TextMessage {
		s.logger.Debug().Msg("a non text message was received, type is " + strconv.Itoa(int(msgType)))
		return nil, nil
	}

	return msg, nil
}

func (s *JsonRpcWebsocket) WriteMessage(msg []byte) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	return s.conn.WriteMessage(s.sessionContext, websocket.TextMessage, msg)
}

func (s *JsonRpcWebsocket) Close() error {
	return s.conn.Close()
}

func (s *JsonRpcWebsocket) Client() string {
	return s.conn.RemoteAddrWithPort().String() + " (remote)"
}
