package jsonrpc

import (
	"context"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strconv"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/inoxlang/inox/internal/lsp/logs"
	jsoniter "github.com/json-iterator/go"
)

type sessionKeyType struct{}

var sessionKey = sessionKeyType{}

type executor struct {
	id     interface{}
	cancel context.CancelFunc
}

type Session struct {
	id     int
	server *Server

	// Only one connection is non-nil
	conn    ReaderWriter
	msgConn MessageReaderWriter

	executors    map[interface{}]*executor
	executorLock sync.Mutex
	writeLock    sync.Mutex
	cancel       chan struct{}
}

func newSessionWithConn(id int, server *Server, conn ReaderWriter) *Session {
	s := &Session{id: id, server: server, conn: conn}
	s.executors = make(map[interface{}]*executor)
	s.cancel = make(chan struct{}, 1)
	return s
}

func newSessionWithMessageConn(id int, server *Server, conn MessageReaderWriter) *Session {
	s := &Session{id: id, server: server, msgConn: conn}
	s.executors = make(map[interface{}]*executor)
	s.cancel = make(chan struct{}, 1)
	return s
}

func (s *Session) Start() {
	for {
		s.handle()
		select {
		case <-s.cancel:
			return
		default:

		}
	}
}

func (s *Session) handle() {
	req, err := s.readRequest()
	if err != nil {
		err := s.handlerResponse(nil, nil, err)
		if err != nil {
			s.handlerError(err)
		}
		return
	}
	logs.Printf("Request: [%v] [%s], content: [%v]\n", req.ID, req.Method, string(req.Params))
	err = s.handlerRequest(req)
	if err != nil {
		err := s.handlerResponse(req.ID, nil, err)
		if err != nil {
			s.handlerError(err)
		}
		return
	}
}

func (s *Session) registerExecutor(executor *executor) {
	s.executorLock.Lock()
	defer s.executorLock.Unlock()
	s.executors[executor.id] = executor
}

func (s *Session) removeExecutor(executor *executor) {
	s.executorLock.Lock()
	defer s.executorLock.Unlock()
	delete(s.executors, executor.id)
}

func (s *Session) readSize(len int) ([]byte, error) {
	reader := s.conn
	buf := make([]byte, len)
	t := 0
	for t != len {
		n, err := reader.Read(buf[t:])
		if err != nil {
			return buf, err
		}
		t += n
	}
	return buf, nil
}

func (s *Session) readRequest() (RequestMessage, error) {
	var contentBytes []byte

	if s.msgConn != nil {
		msg, err := s.msgConn.ReadMessage()
		if err != nil {
			return RequestMessage{}, err
		}
		contentBytes = msg
	} else {
		lenHeader, err := s.readSize(15)
		if err != nil {
			return RequestMessage{}, err
		}
		if strings.ToLower(string(lenHeader)) != "content-length:" {
			return RequestMessage{}, ParseError
		}

		var buf []byte
		state := 0
		for max := 0; max < 20; max++ {
			b, err := s.readSize(1)
			if err != nil {
				return RequestMessage{}, err
			}
			if state == 0 {
				buf = append(buf, b[0])
			} else {
				if b[0] != '\r' && b[0] != '\n' {
					return RequestMessage{}, ParseError
				}
			}
			if b[0] == '\r' {
				if state%2 == 0 {
					state += 1
				} else {
					return RequestMessage{}, ParseError
				}
			}
			if b[0] == '\n' {
				if state%2 == 1 {
					state += 1
					if state == 4 {
						break
					}
				} else {
					return RequestMessage{}, ParseError
				}
			}
		}
		if state != 4 {
			return RequestMessage{}, ParseError
		}

		contentLen, err := strconv.Atoi(strings.TrimSpace(string(buf)))
		if err != nil {
			e := ParseError
			e.Data = err
			return RequestMessage{}, e
		}
		content, err := s.readSize(contentLen)
		if err != nil {
			return RequestMessage{}, err
		}
		contentBytes = content
	}

	req := RequestMessage{}
	err := jsoniter.Unmarshal(contentBytes, &req)
	if err != nil {
		e := ParseError
		e.Data = err
		return RequestMessage{}, e
	}
	return req, nil
}

func GetSession(ctx context.Context) *Session {
	val := ctx.Value(sessionKey)
	if isNil(val) {
		return nil
	}
	return val.(*Session)
}

func (s *Session) getExecutor(id interface{}) *executor {
	if isNil(id) {
		return nil
	}
	s.executorLock.Lock()
	defer s.executorLock.Unlock()
	exec, ok := s.executors[id]
	if !ok {
		return nil
	}
	return exec
}

func (s *Session) cancelJob(id interface{}) {
	exec := s.getExecutor(id)
	if exec == nil {
		return
	}
	exec.cancel()
	s.removeExecutor(exec)
}

func (s *Session) execute(mtdInfo MethodInfo, req RequestMessage, args interface{}) {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	ctx = context.WithValue(ctx, sessionKey, s)
	exec := &executor{
		id:     req.ID,
		cancel: cancel,
	}
	if req.ID != nil {
		s.registerExecutor(exec)
	}
	go func() {
		defer s.removeExecutor(exec)
		resp, err := mtdInfo.Handler(ctx, args)
		select {
		case <-ctx.Done():
			return
		default:
		}
		if isNil(resp) && isNil(err) && isNil(req.ID) {
			return
		}
		err = s.handlerResponse(req.ID, resp, err)
		if err != nil {
			s.handlerError(err)
		}
	}()
}

func (s *Session) handlerRequest(req RequestMessage) error {
	mtd := req.Method
	mtdInfo, ok := s.server.methods[mtd]

	if !ok {
		return MethodNotFound
	}
	reqArgs := mtdInfo.NewRequest()
	err := jsoniter.Unmarshal(req.Params, reqArgs)
	if err != nil {
		return ParseError
	}
	s.execute(mtdInfo, req, reqArgs)
	return nil
}

func (s *Session) write(resp ResponseMessage) error {
	s.writeLock.Lock()
	defer s.writeLock.Unlock()

	res, err := jsoniter.Marshal(resp)
	if err != nil {
		return err
	}

	logs.Printf("Response: [%v] res: [%v]\n", resp.ID, string(res))
	totalLen := len(res)

	if s.msgConn != nil {
		return s.msgConn.WriteMessage(res)
	}

	err = s.mustWrite([]byte(fmt.Sprintf("Content-Length: %d\r\n\r\n", totalLen)))
	if err != nil {
		return err
	}
	err = s.mustWrite(res)
	if err != nil {
		return err
	}
	return nil
}

func (s *Session) Notify(notif NotificationMessage) error {
	s.writeLock.Lock()
	defer s.writeLock.Unlock()

	res, err := jsoniter.Marshal(notif)
	if err != nil {
		return err
	}
	logs.Printf("Notification: [%v]\n", string(res))

	if s.msgConn != nil {
		return s.msgConn.WriteMessage(res)
	}

	totalLen := len(res)
	err = s.mustWrite([]byte(fmt.Sprintf("Content-Length: %d\r\n\r\n", totalLen)))
	if err != nil {
		return err
	}
	err = s.mustWrite(res)
	if err != nil {
		return err
	}
	return nil
}

func (s *Session) mustWrite(data []byte) error {
	t := 0
	for t != len(data) {
		n, err := s.conn.Write(data[t:])
		if err != nil {
			return err
		}
		t += n
	}
	return nil
}
func (s *Session) handlerResponse(id interface{}, result interface{}, err error) error {
	resp := ResponseMessage{ID: id}
	if err != nil {
		if errors.Is(err, io.EOF) {
			return err
		}
		if e, ok := err.(ResponseError); ok {
			resp.Error = &e
		} else {
			return err
		}
	}
	resp.Result = result
	return s.write(resp)
}

func (s *Session) handlerError(err error) {
	isEof := errors.Is(err, io.EOF)
	isWebsocketUnexpectedClose := websocket.IsUnexpectedCloseError(err)

	if isEof || isWebsocketUnexpectedClose {
		// conn done, close conn and remove session

		if s.conn != nil {
			err := s.conn.Close()
			if err != nil {
				logs.Println("close error: ", err)
			}
		} else {
			err := s.msgConn.Close()
			if err != nil {
				logs.Println("message connection: close error: ", err)
			}
		}

		func() {
			s.executorLock.Lock()
			defer s.executorLock.Unlock()
			for _, v := range s.executors {
				if v != nil {
					v.cancel()
				}
			}
		}()

		select {
		case s.cancel <- struct{}{}:
		default:
		}
		s.server.removeSession(s.id)
	}
	logs.Println("error: ", err)
}

func isNil(i interface{}) bool {
	if i == nil {
		return true
	}
	v := reflect.ValueOf(i)
	if v.Kind() == reflect.Ptr && v.IsNil() {
		return true
	}
	return false
}
