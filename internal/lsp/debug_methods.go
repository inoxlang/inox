package internal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"sync"
	"time"

	"github.com/google/go-dap"
	core "github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/inox_ns"
	"github.com/inoxlang/inox/internal/lsp/jsonrpc"
	"github.com/inoxlang/inox/internal/lsp/logs"
	"github.com/inoxlang/inox/internal/lsp/lsp"
	"github.com/inoxlang/inox/internal/utils"
)

type DebugInitializeParams struct {
	SessionId string                `json:"sessionID"`
	Request   dap.InitializeRequest `json:"request"`
}

type DebugConfigurationDoneParams struct {
	SessionId string                       `json:"sessionID"`
	Request   dap.ConfigurationDoneRequest `json:"request"`
}

type DebugLaunchRequestParams struct {
	SessionId string            `json:"sessionID"`
	Request   dap.LaunchRequest `json:"request"`
}

type DebugLaunchArgs struct {
	Program string `json:"program"`
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
	id              string
	nextSeq         int
	programDoneChan chan error //ok if error is nil
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
					Command: dapRequest.Command,
				},
				Body: dap.Capabilities{
					SupportsConfigurationDoneRequest:   true,
					SupportsBreakpointLocationsRequest: true,
				},
			}, nil
		},
	})

	server.OnCustom(jsonrpc.MethodInfo{
		Name: "debug/configurationDone",
		NewRequest: func() interface{} {
			return &DebugConfigurationDoneParams{}
		},
		Handler: func(ctx context.Context, req interface{}) (interface{}, error) {
			session := jsonrpc.GetSession(ctx)
			params := req.(*DebugConfigurationDoneParams)
			dapRequest := params.Request

			debugSession := getDebugSession(session, params.SessionId)

			return dap.ConfigurationDoneResponse{
				Response: dap.Response{
					RequestSeq: dapRequest.Seq,
					Success:    true,
					ProtocolMessage: dap.ProtocolMessage{
						Seq:  debugSession.NextSeq(),
						Type: "response",
					},
					Command: dapRequest.Command,
				},
			}, nil
		},
	})

	server.OnCustom(jsonrpc.MethodInfo{
		Name: "debug/launch",
		NewRequest: func() interface{} {
			return &DebugLaunchRequestParams{}
		},
		Handler: func(ctx context.Context, req interface{}) (interface{}, error) {
			session := jsonrpc.GetSession(ctx)
			sessionCtx := session.Context()
			params := req.(*DebugLaunchRequestParams)
			dapRequest := params.Request

			debugSession := getDebugSession(session, params.SessionId)

			fls, ok := getLspFilesystem(session)
			if !ok {
				return nil, errors.New(FsNoFilesystem)
			}

			var launchArgs DebugLaunchArgs
			err := json.Unmarshal(utils.Must(dapRequest.Arguments.MarshalJSON()), &launchArgs)
			if err != nil {
				return nil, jsonrpc.ResponseError{
					Code:    jsonrpc.InternalError.Code,
					Message: err.Error(),
				}
			}

			if launchArgs.Program == "" {
				if err != nil {
					return dap.LaunchResponse{
						Response: dap.Response{
							RequestSeq: dapRequest.Seq,
							Success:    false,
							ProtocolMessage: dap.ProtocolMessage{
								Seq:  debugSession.NextSeq(),
								Type: "response",
							},
							Message: "missing program in launch arguments",
							Command: dapRequest.Command,
						},
					}, nil
				}
			}

			logs.Println("program: ", launchArgs.Program)
			programPath := filepath.Clean(launchArgs.Program)

			if debugSession.programDoneChan != nil {
				return dap.LaunchResponse{
					Response: dap.Response{
						RequestSeq: dapRequest.Seq,
						Success:    false,
						ProtocolMessage: dap.ProtocolMessage{
							Seq:  debugSession.NextSeq(),
							Type: "response",
						},
						Message: "program is already running",
						Command: dapRequest.Command,
					},
				}, nil
			}

			debugSession.programDoneChan = make(chan error)

			go func() {
				defer func() {
					e := recover()
					switch val := e.(type) {
					case nil:
					case error:
						debugSession.programDoneChan <- val
					default:
						debugSession.programDoneChan <- fmt.Errorf("%#v: %s", val, string(debug.Stack()))
					}
				}()

				ctx := sessionCtx.BoundChildWithOptions(core.BoundChildContextOptions{
					Filesystem: fls,
				})

				_, _, _, err := inox_ns.RunLocalScript(inox_ns.RunScriptArgs{
					Fpath:                     programPath,
					ParsingCompilationContext: ctx,
					ParentContext:             sessionCtx,
					UseContextAsParent:        true,
					Out:                       os.Stdout,
					AllowMissingEnvVars:       false,
					IgnoreHighRiskScore:       true,
					PreinitFilesystem:         fls,
				})

				debugSession.programDoneChan <- err
			}()

			select {
			case <-time.After(time.Second):
				//TODO: only wait for preparation to finish
			case err := <-debugSession.programDoneChan:
				if err != nil {
					return dap.LaunchResponse{
						Response: dap.Response{
							RequestSeq: dapRequest.Seq,
							Success:    false,
							ProtocolMessage: dap.ProtocolMessage{
								Seq:  debugSession.NextSeq(),
								Type: "response",
							},
							Message: "program: " + err.Error(),
							Command: dapRequest.Command,
						},
					}, nil
				}
			}

			return dap.LaunchResponse{
				Response: dap.Response{
					RequestSeq: dapRequest.Seq,
					Success:    true,
					ProtocolMessage: dap.ProtocolMessage{
						Seq:  debugSession.NextSeq(),
						Type: "response",
					},
					Command: dapRequest.Command,
				},
			}, nil
		},
	})

}
