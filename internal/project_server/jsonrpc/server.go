package jsonrpc

import (
	"context"
	"sync"

	core "github.com/inoxlang/inox/internal/core"
)

type MethodInfo struct {
	Name          string
	NewRequest    func() interface{}
	Handler       func(ctx context.Context, req interface{}) (interface{}, error)
	SensitiveData bool

	// List of the maximum number of calls allowed during sliding windows with increasing durations (1s, 10s, and 100s).
	// Example: [10, 50, 200] means at most 10 calls in 1s, 50 calls in 50s and 200 calls in 100s.
	RateLimits []int
}

type Server struct {
	session     map[int]*Session
	nowId       int
	methods     map[string]MethodInfo
	sessionLock sync.Mutex
	onSession   SessionCreationCallbackFn
	ctx         *core.Context
}

// Called before starting each new JSON RPC session.
type SessionCreationCallbackFn func(rpcServerContext *core.Context, session *Session) error

func NewServer(ctx *core.Context, onSession SessionCreationCallbackFn) *Server {
	if onSession == nil {
		onSession = func(ctx *core.Context, s *Session) error { return nil }
	}

	s := &Server{
		onSession: onSession,
		ctx:       ctx,
	}
	s.session = make(map[int]*Session)
	s.methods = make(map[string]MethodInfo)

	// Register Builtin
	s.RegisterMethod(CancelRequest())

	return s
}

func (server *Server) RegisterMethod(m MethodInfo) {
	server.methods[m.Name] = m
}

func (server *Server) ConnComeIn(conn ReaderWriter) {
	session := server.newSession(conn)
	if err := server.onSession(server.ctx, session); err != nil {
		return
	}
	if session.ctx == nil {
		session.ctx = server.ctx.BoundChild()
	}
	session.Start()
}

func (server *Server) MsgConnComeIn(conn MessageReaderWriter, onCreatedSession func(session *Session)) {
	session := server.newSessionWithMsgConn(conn)
	if err := server.onSession(server.ctx, session); err != nil {
		return
	}
	if session.ctx == nil {
		session.ctx = server.ctx.BoundChild()
	}
	if onCreatedSession != nil {
		onCreatedSession(session)
	}
	session.Start()
}

func (s *Server) removeSession(id int) {
	s.sessionLock.Lock()
	defer s.sessionLock.Unlock()
	delete(s.session, id)
}

func (s *Server) newSession(conn ReaderWriter) *Session {
	s.sessionLock.Lock()
	defer s.sessionLock.Unlock()

	id := s.nowId
	s.nowId += 1

	session := newSessionWithConn(id, s, conn)
	s.session[id] = session
	return session
}

func (s *Server) newSessionWithMsgConn(conn MessageReaderWriter) *Session {
	s.sessionLock.Lock()
	defer s.sessionLock.Unlock()

	id := s.nowId
	s.nowId += 1

	session := newSessionWithMessageConn(id, s, conn)
	s.session[id] = session
	return session
}
