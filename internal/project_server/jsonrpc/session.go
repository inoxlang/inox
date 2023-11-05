package jsonrpc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"reflect"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/net_ns"
	"github.com/inoxlang/inox/internal/project_server/logs"
	"github.com/inoxlang/inox/internal/project_server/lsp/defines"
	"github.com/inoxlang/inox/internal/utils"
)

const (
	JSONRPC_VERSION = "2.0"

	MAX_PARAMS_LOGGING_SIZE = 3000
)

var (
	ErrRateLimitedMethod   = errors.New("rate limited method")
	ErrRateLimited         = errors.New("rate limited")
	ErrAlreadyShuttingDown = errors.New("session is already shutting down")

	sessionKey struct{}
)

type Session struct {
	id     int
	server *Server
	ctx    *core.Context

	// Only one connection is non-nil
	conn      ReaderWriter
	msgConn   MessageReaderWriter
	writeLock sync.Mutex

	rateLimiter *rateLimiter

	executors    map[interface{}]*executor
	executorLock sync.Mutex

	closed       atomic.Bool
	shuttingDown atomic.Bool

	closedCallback   func(*Session)
	shutdownCallback func(*Session)
}

type executor struct {
	id     interface{}
	cancel context.CancelFunc
}

func newSessionWithConn(id int, server *Server, conn ReaderWriter) *Session {
	s := newSession(id, server)
	s.conn = conn
	return s
}

func newSessionWithMessageConn(id int, server *Server, conn MessageReaderWriter) *Session {
	s := newSession(id, server)
	s.msgConn = conn
	return s
}

func newSession(id int, server *Server) *Session {
	s := &Session{
		id:          id,
		server:      server,
		rateLimiter: newRateLimiter(*server.ctx.Logger()),
	}
	s.executors = make(map[interface{}]*executor)
	return s
}

func (s *Session) Start() {
	defer s.Close()

	for {
		if continueLoop := s.handle(); !continueLoop {
			return
		}

		if s.closed.Load() {
			return
		}
	}
}

func (s *Session) handle() (continueLoop bool) {
	req, err := s.readRequest()
	if err != nil {
		err := s.handlerResponse(nil, nil, err, false)
		if err != nil {
			return s.handlerError(err)
		}
		return true
	}

	err = s.handlerRequest(req)
	//if error is nil the request is still being processed in another goroutine

	if err != nil {
		err := s.handlerResponse(req.ID, nil, err, false)
		if err != nil {
			return s.handlerError(err)
		}
	}
	return true
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
	err := json.Unmarshal(contentBytes, &req)
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
	stringifiedID := fmt.Sprintf("%v", req.ID)

	if !ok {
		params := req.Params
		suffix := ""
		if len(params) > MAX_PARAMS_LOGGING_SIZE {
			params = params[:min(MAX_PARAMS_LOGGING_SIZE, len(req.Params))]
			suffix = "..." + string(params[len(params)-1])
		}
		logs.Printf("Request: [%v] [%s], content: [%s]%s\n", stringifiedID, req.Method, params, suffix)
		return MethodNotFound
	}

	if mtdInfo.SensitiveData {
		logs.Printf("Request: [%v] [%s], content: ...\n", stringifiedID, req.Method)
	} else {
		params := req.Params
		suffix := ""
		if len(params) > MAX_PARAMS_LOGGING_SIZE {
			params = params[:min(MAX_PARAMS_LOGGING_SIZE, len(req.Params))]
			suffix = "..." + string(params[len(params)-1])
		}
		logs.Printf("Request: [%v] [%s], content: [%s]%s\n", stringifiedID, req.Method, params, suffix)
	}

	if s.IsShuttingDown() && mtdInfo.Name != "exit" {
		return ResponseError{
			Code:    InvalidRequest.Code,
			Message: "session is shutting down",
		}
	}

	//rate limit
	if webSocket, ok := s.msgConn.(*JsonRpcWebsocket); ok {
		addrPort := webSocket.conn.RemoteAddrWithPort()

		if rateLimited, methodRateLimited := s.rateLimiter.limit(mtdInfo, stringifiedID, addrPort); rateLimited {
			if methodRateLimited {
				return fmt.Errorf("%w: %s", ErrRateLimitedMethod, mtdInfo.Name)
			}
			return ErrRateLimited
		}
	}

	reqArgs := mtdInfo.NewRequest()
	if _, ok := reqArgs.(*defines.NoParams); !ok {
		err := json.Unmarshal(req.Params, reqArgs)
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

	res, err := json.Marshal(resp)
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

// SendRequest sends a notification to the client, NotificationMessage.BaseMessage
// is set by the callee.
func (s *Session) Notify(notif NotificationMessage) error {
	notif.BaseMessage = BaseMessage{Jsonrpc: JSONRPC_VERSION}

	s.writeLock.Lock()
	defer s.writeLock.Unlock()

	notifBytes, err := json.Marshal(notif)
	if err != nil {
		return err
	}
	logs.Printf("Notification: [%v]\n", string(notifBytes))

	if s.msgConn != nil {
		return s.msgConn.WriteMessage(notifBytes)
	}

	return s.mustWriteWithContentLengthHeader(notifBytes)
}

// SendRequest sends a request to the client, RequestMessage.ID & RequestMessage.BaseMessage
// are set by the callee.
func (s *Session) SendRequest(req RequestMessage) error {
	req.BaseMessage = BaseMessage{Jsonrpc: JSONRPC_VERSION}
	req.ID = uuid.New()

	s.writeLock.Lock()
	defer s.writeLock.Unlock()

	reqBytes, err := json.Marshal(req)
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

func (s *Session) handlerError(err error) (continueLoop bool) {
	continueLoop = true
	isEof := errors.Is(err, io.EOF)
	isWebsocketUnexpectedClose := websocket.IsUnexpectedCloseError(err)
	isClosedWebsocket := errors.Is(err, net_ns.ErrClosingOrClosedWebsocketConn)
	isNetReaderr := utils.Implements[*net.OpError](err) && err.(*net.OpError).Op == "read"

	if isEof || isWebsocketUnexpectedClose || isClosedWebsocket || isNetReaderr {
		continueLoop = false
	}

	if s.msgConn != nil {
		logs.Printf("error: for client <%s>: %s\n", s.msgConn.Client(), err)
	} else {
		logs.Println("error: ", err)
	}

	return
}

func (s *Session) Context() *core.Context {
	return s.ctx
}

func (s *Session) Client() string {
	if s.msgConn == nil {
		return "(unknown)"
	}
	return s.msgConn.Client()
}

func (s *Session) SetClosedCallbackFn(fn func(session *Session)) {
	if s.closedCallback != nil {
		panic(errors.New("closed callback function already set"))
	}
	s.closedCallback = fn
}

func (s *Session) SetShutdownCallbackFn(fn func(session *Session)) {
	if s.shutdownCallback != nil {
		panic(errors.New("shutdown callback function already set"))
	}
	s.shutdownCallback = fn
}

func (s *Session) IsShuttingDown() bool {
	return s.shuttingDown.Load()
}

func (s *Session) Closed() bool {
	return s.closed.Load()
}

// Close shutdowns the session & closes the connection.
func (s *Session) Close() error {
	if s.closed.Load() {
		return errors.New("already closed")
	}

	defer func() {
		s.closed.Store(true)
		s.shuttingDown.Store(false)
	}()

	//shutdown: call the callback function in another goroutine
	if s.shuttingDown.CompareAndSwap(false, true) {
		if s.shutdownCallback != nil {
			callbackFn := s.shutdownCallback
			s.shutdownCallback = nil

			go func(session *Session) {
				defer utils.Recover()
				callbackFn(session)
			}(s)
		}
	} else {
		return ErrAlreadyShuttingDown
	}

	//defer closed callback
	if s.closedCallback != nil {
		callbackFn := s.closedCallback
		s.closedCallback = nil
		defer func() {
			go func(session *Session) {
				defer utils.Recover()
				callbackFn(session)
			}(s)
		}()
	}
	defer s.ctx.CancelGracefully()

	//close
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

	//cancel all executors
	func() {
		defer utils.Recover()
		s.executorLock.Lock()
		defer s.executorLock.Unlock()

		for _, v := range s.executors {
			if v != nil {
				v.cancel()
			}
		}
	}()

	s.server.removeSession(s.id)
	return nil
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
