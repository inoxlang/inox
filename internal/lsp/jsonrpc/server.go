package jsonrpc

import (
	"context"
	"sync"
)

type MethodInfo struct {
	Name       string
	NewRequest func() interface{}
	Handler    func(ctx context.Context, req interface{}) (interface{}, error)
}

type Server struct {
	session     map[int]*Session
	nowId       int
	methods     map[string]MethodInfo
	sessionLock sync.Mutex
	onSession   SessionCreationCallbackFn
}

// Called before starting each new JSON RPC session.
type SessionCreationCallbackFn func(*Session) error

func NewServer(onSession func(*Session) error) *Server {
	if onSession == nil {
		onSession = func(s *Session) error { return nil }
	}

	s := &Server{
		onSession: onSession,
	}
	s.session = make(map[int]*Session)
	s.methods = make(map[string]MethodInfo)

	// Register Builtin
	s.RegisterMethod(CancelRequest())

	return s
}

func (s *Server) RegisterMethod(m MethodInfo) {
	s.methods[m.Name] = m
}

func (s *Server) ConnComeIn(conn ReaderWriter) {
	session := s.newSession(conn)
	if err := s.onSession(session); err != nil {
		return
	}
	session.Start()
}

func (s *Server) MsgConnComeIn(conn MessageReaderWriter) {
	session := s.newSessionWithMsgConn(conn)
	if err := s.onSession(session); err != nil {
		return
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
