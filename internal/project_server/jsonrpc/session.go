package jsonrpc

import (
	"context"
	"errors"
	"fmt"
	"io"
	"reflect"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"

	"github.com/gorilla/websocket"

	core "github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/net_ns"
	"github.com/inoxlang/inox/internal/project_server/logs"
	"github.com/inoxlang/inox/internal/project_server/lsp/defines"
	"github.com/inoxlang/inox/internal/utils"
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
	ctx    *core.Context

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
		err := s.handlerResponse(nil, nil, err, false)
		if err != nil {
			s.handlerError(err)
		}
		return
	}

	err = s.handlerRequest(req)
	if err != nil {
		err := s.handlerResponse(req.ID, nil, err, false)
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
	ctx, cancel := context.WithCancel(s.ctx)
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
		defer func() {
			if e := recover(); e != nil {
				err := utils.ConvertPanicValueToError(e)
				logs.Println(fmt.Errorf("%w: %s", err, string(debug.Stack())))
			}
		}()

		resp, err := mtdInfo.Handler(ctx, args)

		select {
		case <-ctx.Done():
			return
		default:
		}

		if isNil(resp) && isNil(err) && isNil(req.ID) {
			return
		}
		err = s.handlerResponse(req.ID, resp, err, mtdInfo.SensitiveData)
		if err != nil {
			s.handlerError(err)
		}
	}()
}

func (s *Session) handlerRequest(req RequestMessage) error {
	mtd := req.Method
	mtdInfo, ok := s.server.methods[mtd]

	if !ok {
		logs.Printf("Request: [%v] [%s], content: [%v]\n", req.ID, req.Method, string(req.Params))
		return MethodNotFound
	}

	if mtdInfo.SensitiveData {
		logs.Printf("Request: [%v] [%s], content: ...\n", req.ID, req.Method)
	} else {
		logs.Printf("Request: [%v] [%s], content: [%v]\n", req.ID, req.Method, string(req.Params))
	}

	reqArgs := mtdInfo.NewRequest()
	if _, ok := reqArgs.(*defines.NoParams); !ok {
		err := jsoniter.Unmarshal(req.Params, reqArgs)
		if err != nil {
			return ParseError
		}
	}

	s.execute(mtdInfo, req, reqArgs)
	return nil
}

func (s *Session) write(resp ResponseMessage, sensitiveMethod bool) error {
	s.writeLock.Lock()
	defer s.writeLock.Unlock()

	res, err := jsoniter.Marshal(resp)
	if err != nil {
		return err
	}

	if sensitiveMethod {
		logs.Printf("Response: [%v] res: ...\n", resp.ID)
	} else {
		logs.Printf("Response: [%v] res: [%v]\n", resp.ID, string(res))
	}

	if s.msgConn != nil {
		return s.msgConn.WriteMessage(res)
	}

	return s.mustWriteWithContentLengthHeader(res)
}

func (s *Session) Notify(notif NotificationMessage) error {
	s.writeLock.Lock()
	defer s.writeLock.Unlock()

	notifBytes, err := jsoniter.Marshal(notif)
	if err != nil {
		return err
	}
	logs.Printf("Notification: [%v]\n", string(notifBytes))

	if s.msgConn != nil {
		return s.msgConn.WriteMessage(notifBytes)
	}

	return s.mustWriteWithContentLengthHeader(notifBytes)
}

func (s *Session) SendRequest(req RequestMessage) error {
	s.writeLock.Lock()
	defer s.writeLock.Unlock()

	reqBytes, err := jsoniter.Marshal(req)
	if err != nil {
		return err
	}
	logs.Printf("Request To Client: [%v]\n", string(reqBytes))

	if s.msgConn != nil {
		return s.msgConn.WriteMessage(reqBytes)
	}

	return s.mustWriteWithContentLengthHeader(reqBytes)
}

func (s *Session) mustWriteWithContentLengthHeader(msg []byte) error {
	totalLen := len(msg)

	err := s.mustWrite([]byte(fmt.Sprintf("Content-Length: %d\r\n\r\n", totalLen)))
	if err != nil {
		return err
	}
	err = s.mustWrite(msg)
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
func (s *Session) handlerResponse(id interface{}, result interface{}, err error, sensitiveDataMethod bool) error {
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
	return s.write(resp, sensitiveDataMethod)
}

func (s *Session) handlerError(err error) {
	isEof := errors.Is(err, io.EOF)
	isWebsocketUnexpectedClose := websocket.IsUnexpectedCloseError(err)
	isClosedWebsocket := errors.Is(err, net_ns.ErrClosedWebsocketConnection)

	if isEof || isWebsocketUnexpectedClose || isClosedWebsocket {
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

func (s *Session) Context() *core.Context {
	return s.ctx
}

func (s *Session) Close() error {
	defer s.ctx.Cancel()

	if s.conn != nil {
		return s.conn.Close()
	} else {
		return s.msgConn.Close()
	}
}

func (s *Session) SetContextOnce(ctx *core.Context) error {
	if s.ctx != nil {
		return errors.New("already set")
	}
	s.ctx = ctx
	return nil
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
