package projectserver

import (
	"slices"
	"sync"
	"sync/atomic"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/projectserver/jsonrpc"
	"github.com/inoxlang/inox/internal/projectserver/lsp/defines"
)

func getDebugSession(session *jsonrpc.Session, sessionId string) (*DebugSession, error) {
	sessionData := getLockedSessionData(session)

	debugSessions := sessionData.debugSessions
	if debugSessions == nil {
		debugSessions = &DebugSessions{}
		sessionData.debugSessions = debugSessions
	}
	sessionData.lock.Unlock()

	debugSession, ok := debugSessions.GetSession(sessionId)
	if !ok {
		return nil, ErrUnknowSessionId
	}

	return debugSession, nil
}

func createDebugSession(session *jsonrpc.Session, sessionId string) (*DebugSession, error) {
	sessionData := getLockedSessionData(session)

	debugSessions := sessionData.debugSessions
	if debugSessions == nil {
		debugSessions = &DebugSessions{}
		sessionData.debugSessions = debugSessions
	}
	sessionData.lock.Unlock()

	if len(debugSessions.sessions) >= DEFAULT_MAX_SESSION_COUNT {
		return nil, ErrMaxParallelDebugSessionReached
	}

	var debugSession *DebugSession
	for _, s := range debugSessions.sessions {
		if s.id == sessionId {
			return nil, ErrSessionAlreadyExists
		}
	}

	debugSession = &DebugSession{
		id:                             sessionId,
		sourcePathToInitialBreakpoints: make(map[string][]core.BreakpointInfo),
		nextInitialBreakpointId:        core.INITIAL_BREAKPOINT_ID,
		inProjectMode:                  sessionData.projectMode,

		variablesReferences: make(map[core.StateId]*variablesReferences, 0),
	}
	debugSession.nextSeq.Store(1)
	debugSessions.AddSession(debugSession)

	return debugSession, nil
}

func removeDebugSession(debugSession *DebugSession, session *jsonrpc.Session) {
	sessionData := getLockedSessionData(session)
	debugSessions := sessionData.debugSessions
	sessionData.lock.Unlock()
	if debugSessions != nil {
		debugSessions.RemoveSession(debugSession)
	}
}

type DebugSessions struct {
	sessions        []*DebugSession
	sessionListLock sync.Mutex
}

// TODO: limit running sessions to 2.
func (sessions *DebugSessions) AddSession(s *DebugSession) {
	sessions.sessionListLock.Lock()
	defer sessions.sessionListLock.Unlock()
	sessions.sessions = append(sessions.sessions, s)
}

func (sessions *DebugSessions) GetSession(sessionId string) (*DebugSession, bool) {
	sessions.sessionListLock.Lock()
	defer sessions.sessionListLock.Unlock()

	for _, s := range sessions.sessions {
		if s.id == sessionId {
			return s, true
		}
	}
	return nil, false
}

func (sessions *DebugSessions) RemoveSession(s *DebugSession) {
	sessions.sessionListLock.Lock()
	defer sessions.sessionListLock.Unlock()
	sessions.sessions = slices.DeleteFunc(sessions.sessions, func(ds *DebugSession) bool {
		return ds == s
	})
}

type DebugSession struct {
	id                             string
	programPath                    string
	programURI                     defines.DocumentUri
	columnsStartAt1, lineStartsAt1 bool
	configurationDone              atomic.Bool
	inProjectMode                  bool

	//initial breakpoints
	//this field is set to nil during launch to remove some unecessary references
	sourcePathToInitialBreakpoints map[string][]core.BreakpointInfo
	initialExceptionBreakpointsId  int32
	nextInitialBreakpointId        int32
	initialBreakpointsLock         sync.Mutex

	debugger                *core.Debugger //set during or shorty after the 'debug/launch' call
	debuggerSet             atomic.Bool
	nextSeq                 atomic.Int32
	variablesReferences     map[core.StateId]*variablesReferences
	variablesReference      atomic.Int32
	variablesReferencesLock sync.Mutex

	programDoneChan               chan error //ok if error is nil
	programPreparedOrFailedToChan chan error
	wasAttached                   bool //debugger was attached to a running debuggee
	finished                      atomic.Bool
	receivedDisconnectRequest     atomic.Bool
}

type variablesReferences struct {
	//set at creation, access does not require locking
	localScope  int
	globalScope int

	//
	lock sync.Mutex
}

func (s *DebugSession) NextSeq() int {
	next := s.nextSeq.Add(1)

	return int(next - 1)
}

func (s *DebugSession) getThreadVariablesReferences(id core.StateId) *variablesReferences {
	s.variablesReferencesLock.Lock()
	defer s.variablesReferencesLock.Unlock()

	refs := s.variablesReferences[id]
	if refs == nil {
		refs = &variablesReferences{
			localScope:  int(s.variablesReference.Add(1)),
			globalScope: int(s.variablesReference.Add(1)),
		}
		s.variablesReferences[id] = refs
	}

	return refs
}

func (s *DebugSession) getThreadOfVariablesReference(varsRef int) (core.StateId, *variablesReferences, bool) {
	s.variablesReferencesLock.Lock()
	defer s.variablesReferencesLock.Unlock()

	for threadId, refs := range s.variablesReferences {

		if refs.localScope == varsRef || refs.globalScope == varsRef {
			return threadId, refs, true
		}

	}
	return 0, nil, false
}
