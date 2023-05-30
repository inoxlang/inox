package lsp

import (
	"strconv"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/inoxlang/inox/internal/lsp/jsonrpc"
	"github.com/rs/zerolog"
)

const (
	CONTENT_LENGTH_HEADER = "Content-Length: "
)

var (
	_ jsonrpc.MessageReaderWriter = (*JsonRpcWebsocket)(nil)
)

type JsonRpcWebsocket struct {
	conn   *websocket.Conn
	lock   sync.RWMutex
	logger zerolog.Logger
}

func NewJsonRpcWebsocket(conn *websocket.Conn, logger zerolog.Logger) *JsonRpcWebsocket {
	return &JsonRpcWebsocket{conn: conn}
}

func (s *JsonRpcWebsocket) ReadMessage() ([]byte, error) {
	msgType, msg, err := s.conn.ReadMessage()
	if err != nil {
		s.logger.Err(err).Msg("error while reading message from websocket")
		return nil, err
	}

	if msgType != websocket.TextMessage {
		s.logger.Debug().Msg("a non text message was received, type is " + strconv.Itoa(msgType))
		return nil, nil
	}

	return msg, nil
}

func (s *JsonRpcWebsocket) WriteMessage(msg []byte) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	return s.conn.WriteMessage(websocket.TextMessage, msg)
}

func (s *JsonRpcWebsocket) Close() error {
	return s.conn.Close()
}
