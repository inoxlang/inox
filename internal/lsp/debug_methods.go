package internal

import (
	"context"
	"sync"

	"github.com/google/go-dap"
	"github.com/inoxlang/inox/internal/lsp/jsonrpc"
	"github.com/inoxlang/inox/internal/lsp/lsp"
)

type DebugInitializeParams struct {
	SessionId string                `json:"sessionID"`
	Request   dap.InitializeRequest `json:"request"`
}

type DebugSessions struct {
	sessions        []*DebugSession
	sessionListLock sync.Mutex
}

func (sessions *DebugSessions) AddSession(s *DebugSession) {
	sessions.sessionListLock.Lock()
	defer sessions.sessionListLock.Unlock()
	sessions.sessions = append(sessions.sessions, s)
}

type DebugSession struct {
	id      string
	nextSeq int
}

func (s *DebugSession) NextSeq() int {
	seq := s.nextSeq
	s.nextSeq++
	return seq
}

func registerDebugMethodHandlers(
	server *lsp.Server, opts LSPServerOptions,
	sessionToDebugSessions map[*jsonrpc.Session]*DebugSessions, sessionToDebugSessionsLock *sync.Mutex,
) {

	getDebugSession := func(session *jsonrpc.Session, sessionId string) *DebugSession {
		sessionToDebugSessionsLock.Lock()
		debugSessions, ok := sessionToDebugSessions[session]
		if !ok {
			debugSessions = &DebugSessions{}
			sessionToDebugSessions[session] = debugSessions
		}
		sessionToDebugSessionsLock.Unlock()

		var debugSession *DebugSession
		for _, s := range debugSessions.sessions {
			if s.id == sessionId {
				debugSession = s
			}
		}

		if debugSession == nil {
			debugSession = &DebugSession{
				id:      sessionId,
				nextSeq: 1,
			}
			debugSessions.AddSession(debugSession)
		}

		return debugSession
	}

	server.OnCustom(jsonrpc.MethodInfo{
		Name: "debug/initialize",
		NewRequest: func() interface{} {
			return &DebugInitializeParams{}
		},
		Handler: func(ctx context.Context, req interface{}) (interface{}, error) {
			session := jsonrpc.GetSession(ctx)
			params := req.(*DebugInitializeParams)
			dapRequest := params.Request

			debugSession := getDebugSession(session, params.SessionId)

			return dap.InitializeResponse{
				Response: dap.Response{
					RequestSeq: dapRequest.Seq,
					Success:    true,
					ProtocolMessage: dap.ProtocolMessage{
						Seq:  debugSession.NextSeq(),
						Type: "response",
					},
					Command: "initialize",
				},
				Body: dap.Capabilities{
					SupportsConfigurationDoneRequest:   true,
					SupportsBreakpointLocationsRequest: true,
				},
			}, nil
		},
	})

}
