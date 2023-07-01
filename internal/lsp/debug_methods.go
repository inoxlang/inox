package internal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/go-dap"
	core "github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/inox_ns"
	"github.com/inoxlang/inox/internal/lsp/jsonrpc"
	"github.com/inoxlang/inox/internal/lsp/logs"
	"github.com/inoxlang/inox/internal/lsp/lsp"
	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/rs/zerolog"
)

const (
	DEFAULT_DEBUG_COMMAND_TIMEOUT = 2 * time.Second
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

type DebugPauseParams struct {
	SessionId string           `json:"sessionID"`
	Request   dap.PauseRequest `json:"request"`
}

type DebugContinueParams struct {
	SessionId string              `json:"sessionID"`
	Request   dap.ContinueRequest `json:"request"`
}

type DebugThreadsParams struct {
	SessionId string             `json:"sessionID"`
	Request   dap.ThreadsRequest `json:"request"`
}

type DebugStackTraceParams struct {
	SessionId string                `json:"sessionID"`
	Request   dap.StackTraceRequest `json:"request"`
}

type DebugScopesParams struct {
	SessionId string            `json:"sessionID"`
	Request   dap.ScopesRequest `json:"request"`
}

type DebugVariablesParams struct {
	SessionId string               `json:"sessionID"`
	Request   dap.VariablesRequest `json:"request"`
}

type DebugSetBreakpointsParams struct {
	SessionId string                    `json:"sessionID"`
	Request   dap.SetBreakpointsRequest `json:"request"`
}

type DebugLaunchArgs struct {
	Program string `json:"program"`
}

type DebugDisconnectParams struct {
	SessionId string                `json:"sessionID"`
	Request   dap.DisconnectRequest `json:"request"`
}

type DebugSecondaryEvent struct {
	dap.Event
	Body any `json:"body"`
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

type DebugSession struct {
	id      string
	nextSeq atomic.Int32

	programPath                    string
	columnsStartAt1, lineStartsAt1 bool
	configurationDone              atomic.Bool

	//this field is set to nil during launch to remove some unecessary references
	sourcePathToInitialBreakpoints map[string][]core.BreakpointInfo
	nextInitialBreakpointId        int32
	initialBreakpointsLock         sync.Mutex

	debugger                      *core.Debugger
	wasAttached                   bool       //debugger was attached to a running debuggee
	programDoneChan               chan error //ok if error is nil
	programPreparedOrFailedToChan chan error
	finished                      atomic.Bool
}

func (s *DebugSession) NextSeq() int {
	next := s.nextSeq.Add(1)

	return int(next - 1)
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
				id:                             sessionId,
				sourcePathToInitialBreakpoints: make(map[string][]core.BreakpointInfo),
				nextInitialBreakpointId:        core.INITIAL_BREAKPOINT_ID,
			}
			debugSession.nextSeq.Store(1)
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

			debugSession.columnsStartAt1 = dapRequest.Arguments.ColumnsStartAt1
			debugSession.lineStartsAt1 = dapRequest.Arguments.LinesStartAt1

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
					SupportsConfigurationDoneRequest: true,
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

			if !debugSession.configurationDone.CompareAndSwap(false, true) {
				return dap.ConfigurationDoneResponse{
					Response: dap.Response{
						RequestSeq: dapRequest.Seq,
						Success:    false,
						ProtocolMessage: dap.ProtocolMessage{
							Seq:  debugSession.NextSeq(),
							Type: "response",
						},
						Command: dapRequest.Command,
						Message: "configuration is already done",
					},
				}, nil
			}

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
			params := req.(*DebugLaunchRequestParams)
			dapRequest := params.Request

			debugSession := getDebugSession(session, params.SessionId)

			fls, ok := getLspFilesystem(session)
			if !ok {
				return nil, errors.New(FsNoFilesystem)
			}

			if !debugSession.configurationDone.Load() {
				return dap.LaunchResponse{
					Response: dap.Response{
						RequestSeq: dapRequest.Seq,
						Success:    false,
						ProtocolMessage: dap.ProtocolMessage{
							Seq:  debugSession.NextSeq(),
							Type: "response",
						},
						Message: "failed to launch: configuration is not done",
						Command: dapRequest.Command,
					},
				}, nil
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

			debugSession.programPath = programPath
			debugSession.programDoneChan = make(chan error, 1)
			debugSession.programPreparedOrFailedToChan = make(chan error)

			go launchDebuggedProgram(programPath, session, debugSession, fls)

			err = <-debugSession.programPreparedOrFailedToChan
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

	server.OnCustom(jsonrpc.MethodInfo{
		Name: "debug/threads",
		NewRequest: func() interface{} {
			return &DebugThreadsParams{}
		},
		Handler: func(ctx context.Context, req interface{}) (interface{}, error) {
			session := jsonrpc.GetSession(ctx)
			params := req.(*DebugThreadsParams)
			dapRequest := params.Request

			debugSession := getDebugSession(session, params.SessionId)

			var threads []dap.Thread

			threads = append(threads, dap.Thread{
				Id:   1,
				Name: debugSession.programPath,
			})

			return dap.ThreadsResponse{
				Response: dap.Response{
					RequestSeq: dapRequest.Seq,
					Success:    true,
					ProtocolMessage: dap.ProtocolMessage{
						Seq:  debugSession.NextSeq(),
						Type: "response",
					},
					Command: dapRequest.Command,
				},
				Body: dap.ThreadsResponseBody{
					Threads: threads,
				},
			}, nil
		},
	})

	server.OnCustom(jsonrpc.MethodInfo{
		Name: "debug/stackTrace",
		NewRequest: func() interface{} {
			return &DebugStackTraceParams{}
		},
		Handler: func(ctx context.Context, req interface{}) (interface{}, error) {
			session := jsonrpc.GetSession(ctx)
			params := req.(*DebugStackTraceParams)
			dapRequest := params.Request

			debugSession := getDebugSession(session, params.SessionId)

			var stackFrames []dap.StackFrame
			framesChan := make(chan []dap.StackFrame)

			debugSession.debugger.ControlChan() <- core.DebugCommandGetStackTrace{
				Get: func(trace []core.StackFrameInfo) {
					var frames []dap.StackFrame

					for _, frame := range trace {
						var source *dap.Source
						src, ok := frame.Chunk.Source.(parse.SourceFile)
						if ok && !src.IsResourceURL {
							source = &dap.Source{
								Name: src.Name(),
								Path: INOX_FS_SCHEME + "://" + src.Resource,
							}
						}

						frames = append(frames, dap.StackFrame{
							Id:     int(frame.Id),
							Name:   frame.Name,
							Source: source,
							Line:   int(frame.StatementStartLine),
							Column: int(frame.StatementStartColumn),
						})
					}

					framesChan <- frames
				},
			}

			select {
			case stackFrames = <-framesChan:
			case <-time.After(DEFAULT_DEBUG_COMMAND_TIMEOUT):
				return dap.StackTraceResponse{
					Response: dap.Response{
						RequestSeq: dapRequest.Seq,
						Success:    false,
						ProtocolMessage: dap.ProtocolMessage{
							Seq:  debugSession.NextSeq(),
							Type: "response",
						},
						Message: "failed to get stack trace",
						Command: dapRequest.Command,
					},
				}, nil
			}

			totalFrames := len(stackFrames)
			stackFrames = stackFrames[dapRequest.Arguments.StartFrame:]
			maxFrames := dapRequest.Arguments.Levels
			if maxFrames > 0 {
				stackFrames = stackFrames[:utils.Min(len(stackFrames), maxFrames)]
			}

			return dap.StackTraceResponse{
				Response: dap.Response{
					RequestSeq: dapRequest.Seq,
					Success:    true,
					ProtocolMessage: dap.ProtocolMessage{
						Seq:  debugSession.NextSeq(),
						Type: "response",
					},
					Command: dapRequest.Command,
				},
				Body: dap.StackTraceResponseBody{
					StackFrames: stackFrames,
					TotalFrames: totalFrames,
				},
			}, nil
		},
	})

	server.OnCustom(jsonrpc.MethodInfo{
		Name: "debug/scopes",
		NewRequest: func() interface{} {
			return &DebugScopesParams{}
		},
		Handler: func(ctx context.Context, req interface{}) (interface{}, error) {
			session := jsonrpc.GetSession(ctx)
			params := req.(*DebugScopesParams)
			dapRequest := params.Request

			debugSession := getDebugSession(session, params.SessionId)

			var scopes []dap.Scope
			scopesChan := make(chan []dap.Scope)

			debugSession.debugger.ControlChan() <- core.DebugCommandGetScopes{
				Get: func(globalScope, localScope map[string]core.Value) {
					scopesChan <- []dap.Scope{
						{
							Name:               "Local Scope",
							PresentationHint:   "locals",
							NamedVariables:     len(localScope),
							VariablesReference: 1000,
						},
						{
							Name:               "Global Scope",
							NamedVariables:     len(globalScope),
							VariablesReference: 1,
						},
					}
				},
			}

			select {
			case scopes = <-scopesChan:
			case <-time.After(DEFAULT_DEBUG_COMMAND_TIMEOUT):
				return dap.ScopesResponse{
					Response: dap.Response{
						RequestSeq: dapRequest.Seq,
						Success:    false,
						ProtocolMessage: dap.ProtocolMessage{
							Seq:  debugSession.NextSeq(),
							Type: "response",
						},
						Message: "failed to get scopes",
						Command: dapRequest.Command,
					},
				}, nil
			}

			return dap.ScopesResponse{
				Response: dap.Response{
					RequestSeq: dapRequest.Seq,
					Success:    true,
					ProtocolMessage: dap.ProtocolMessage{
						Seq:  debugSession.NextSeq(),
						Type: "response",
					},
					Command: dapRequest.Command,
				},
				Body: dap.ScopesResponseBody{
					Scopes: scopes,
				},
			}, nil
		},
	})

	server.OnCustom(jsonrpc.MethodInfo{
		Name: "debug/variables",
		NewRequest: func() interface{} {
			return &DebugVariablesParams{}
		},
		Handler: func(ctx context.Context, req interface{}) (interface{}, error) {
			session := jsonrpc.GetSession(ctx)
			params := req.(*DebugVariablesParams)
			dapRequest := params.Request

			debugSession := getDebugSession(session, params.SessionId)

			var variables []dap.Variable
			varsChan := make(chan []dap.Variable)

			ref := dapRequest.Arguments.VariablesReference

			debugSession.debugger.ControlChan() <- core.DebugCommandGetScopes{
				Get: func(globalScope, localScope map[string]core.Value) {
					var variables []dap.Variable

					handlingCtx := session.Context().BoundChild()

					var scope map[string]core.Value

					switch ref {
					case 1:
						scope = globalScope
					case 1000:
						scope = localScope
					default:
						//invalid reference
					}

					for k, v := range scope {
						variables = append(variables, dap.Variable{
							Name:  k,
							Value: core.Stringify(v, handlingCtx),
						})
					}
					varsChan <- variables
				},
			}

			select {
			case variables = <-varsChan:
			case <-time.After(DEFAULT_DEBUG_COMMAND_TIMEOUT):
				return dap.VariablesResponse{
					Response: dap.Response{
						RequestSeq: dapRequest.Seq,
						Success:    false,
						ProtocolMessage: dap.ProtocolMessage{
							Seq:  debugSession.NextSeq(),
							Type: "response",
						},
						Message: "failed to get variables",
						Command: dapRequest.Command,
					},
				}, nil
			}

			return dap.VariablesResponse{
				Response: dap.Response{
					RequestSeq: dapRequest.Seq,
					Success:    true,
					ProtocolMessage: dap.ProtocolMessage{
						Seq:  debugSession.NextSeq(),
						Type: "response",
					},
					Command: dapRequest.Command,
				},
				Body: dap.VariablesResponseBody{
					Variables: variables,
				},
			}, nil
		},
	})

	server.OnCustom(jsonrpc.MethodInfo{
		Name: "debug/setBreakpoints",
		NewRequest: func() interface{} {
			return &DebugSetBreakpointsParams{}
		},
		Handler: func(ctx context.Context, req interface{}) (interface{}, error) {
			session := jsonrpc.GetSession(ctx)
			params := req.(*DebugSetBreakpointsParams)
			dapRequest := params.Request

			debugSession := getDebugSession(session, params.SessionId)

			pathWithScheme := dapRequest.Arguments.Source.Path
			path := strings.TrimPrefix(pathWithScheme, INOX_FS_SCHEME+":")

			if pathWithScheme == "" {
				return dap.SetBreakpointsResponse{
					Response: dap.Response{
						RequestSeq: dapRequest.Seq,
						Success:    false,
						ProtocolMessage: dap.ProtocolMessage{
							Seq:  debugSession.NextSeq(),
							Type: "response",
						},
						Message: "source.path is not set",
						Command: dapRequest.Command,
					},
				}, nil
			}

			var lines []int

			for _, srcBreakpoint := range dapRequest.Arguments.Breakpoints {
				lines = append(lines, srcBreakpoint.Line)
			}

			//read & parse file
			fls, ok := getLspFilesystem(session)
			if !ok {
				return nil, errors.New(FsNoFilesystem)
			}

			chunk, err := core.ParseFileChunk(path, fls)

			if err != nil {
				return dap.SetBreakpointsResponse{
					Response: dap.Response{
						RequestSeq: dapRequest.Seq,
						Success:    false,
						ProtocolMessage: dap.ProtocolMessage{
							Seq:  debugSession.NextSeq(),
							Type: "response",
						},
						Message: fmt.Sprintf("failed to get read/parse %s: %s", pathWithScheme, err),
						Command: dapRequest.Command,
					},
				}, nil
			}

			//initial breakpoints
			if !debugSession.configurationDone.Load() {
				var (
					breakpoints []core.BreakpointInfo
					err         error
				)

				func() {
					debugSession.initialBreakpointsLock.Lock()
					defer debugSession.initialBreakpointsLock.Unlock()
					nextBreakpointId := &debugSession.nextInitialBreakpointId

					breakpoints, err = core.GetBreakpointsFromLines(lines, chunk, nextBreakpointId)
				}()

				//get breakpoints & return them in the response

				if err != nil {
					debugSession.initialBreakpointsLock.Unlock()
					return dap.SetBreakpointsResponse{
						Response: dap.Response{
							RequestSeq: dapRequest.Seq,
							Success:    false,
							ProtocolMessage: dap.ProtocolMessage{
								Seq:  debugSession.NextSeq(),
								Type: "response",
							},
							Message: fmt.Sprintf("failed to get breakpoints by line in %s: %s", pathWithScheme, err),
							Command: dapRequest.Command,
						},
					}, nil
				}
				debugSession.initialBreakpointsLock.Lock()
				debugSession.sourcePathToInitialBreakpoints[path] = breakpoints
				debugSession.initialBreakpointsLock.Unlock()

				var dapBreakpoints []dap.Breakpoint
				for _, breakpoint := range breakpoints {
					dapBreakpoint := breakpointInfoToDebugAdapterProtocolBreakpoint(breakpoint, debugSession.columnsStartAt1)
					dapBreakpoints = append(dapBreakpoints, dapBreakpoint)
				}

				return dap.SetBreakpointsResponse{
					Response: dap.Response{
						RequestSeq: dapRequest.Seq,
						Success:    true,
						ProtocolMessage: dap.ProtocolMessage{
							Seq:  debugSession.NextSeq(),
							Type: "response",
						},
						Command: dapRequest.Command,
					},
					Body: dap.SetBreakpointsResponseBody{
						Breakpoints: dapBreakpoints,
					},
				}, nil
			}

			//else non-initial breakpoints (program is launched)

			breakpointsChan := make(chan []dap.Breakpoint)

			cmd := core.DebugCommandSetBreakpoints{
				Chunk:             chunk,
				BreakPointsByLine: lines,
				GetBreakpointsSetByLine: func(breakpoints []core.BreakpointInfo) {
					var dapBreakpoints []dap.Breakpoint
					for _, breakpoint := range breakpoints {
						dapBreakpoint := breakpointInfoToDebugAdapterProtocolBreakpoint(breakpoint, debugSession.columnsStartAt1)
						dapBreakpoints = append(dapBreakpoints, dapBreakpoint)
					}
					breakpointsChan <- dapBreakpoints
				},
			}

			debugSession.debugger.ControlChan() <- cmd

			var breakpoints []dap.Breakpoint

			select {
			case breakpoints = <-breakpointsChan:
			case <-time.After(DEFAULT_DEBUG_COMMAND_TIMEOUT):
				return dap.SetBreakpointsResponse{
					Response: dap.Response{
						RequestSeq: dapRequest.Seq,
						Success:    false,
						ProtocolMessage: dap.ProtocolMessage{
							Seq:  debugSession.NextSeq(),
							Type: "response",
						},
						Message: "failed to set breakpoints",
						Command: dapRequest.Command,
					},
				}, nil
			}

			return dap.SetBreakpointsResponse{
				Response: dap.Response{
					RequestSeq: dapRequest.Seq,
					Success:    true,
					ProtocolMessage: dap.ProtocolMessage{
						Seq:  debugSession.NextSeq(),
						Type: "response",
					},
					Command: dapRequest.Command,
				},
				Body: dap.SetBreakpointsResponseBody{
					Breakpoints: breakpoints,
				},
			}, nil
		},
	})

	server.OnCustom(jsonrpc.MethodInfo{
		Name: "debug/pause",
		NewRequest: func() interface{} {
			return &DebugPauseParams{}
		},
		Handler: func(ctx context.Context, req interface{}) (interface{}, error) {
			session := jsonrpc.GetSession(ctx)
			params := req.(*DebugPauseParams)
			dapRequest := params.Request

			debugSession := getDebugSession(session, params.SessionId)

			debugger := debugSession.debugger
			if !debugger.Closed() {
				debugger.ControlChan() <- core.DebugCommandPause{}
			}

			return dap.PauseResponse{
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
		Name: "debug/continue",
		NewRequest: func() interface{} {
			return &DebugContinueParams{}
		},
		Handler: func(ctx context.Context, req interface{}) (interface{}, error) {
			session := jsonrpc.GetSession(ctx)
			params := req.(*DebugContinueParams)
			dapRequest := params.Request

			debugSession := getDebugSession(session, params.SessionId)

			debugger := debugSession.debugger
			if !debugger.Closed() {
				//TODO: support continuing a specific thread (see params)
				debugger.ControlChan() <- core.DebugCommandContinue{}
			}

			return dap.ContinueResponse{
				Response: dap.Response{
					RequestSeq: dapRequest.Seq,
					Success:    true,
					ProtocolMessage: dap.ProtocolMessage{
						Seq:  debugSession.NextSeq(),
						Type: "response",
					},
					Command: dapRequest.Command,
				},
				Body: dap.ContinueResponseBody{
					AllThreadsContinued: true,
				},
			}, nil
		},
	})

	server.OnCustom(jsonrpc.MethodInfo{
		Name: "debug/disconnect",
		NewRequest: func() interface{} {
			return &DebugDisconnectParams{}
		},
		Handler: func(ctx context.Context, req interface{}) (interface{}, error) {
			session := jsonrpc.GetSession(ctx)
			params := req.(*DebugDisconnectParams)
			dapRequest := params.Request

			debugSession := getDebugSession(session, params.SessionId)
			debugger := debugSession.debugger

			doneChan := make(chan struct{})

			if debugger != nil && !debugger.Closed() {
				debugger.ControlChan() <- core.DebugCommandCloseDebugger{
					CancelExecution: !debugSession.wasAttached,
					Done: func() {
						doneChan <- struct{}{}
					},
				}

				select {
				case <-doneChan:
				case <-time.After(DEFAULT_DEBUG_COMMAND_TIMEOUT):
					return dap.DisconnectResponse{
						Response: dap.Response{
							RequestSeq: dapRequest.Seq,
							Success:    false,
							ProtocolMessage: dap.ProtocolMessage{
								Seq:  debugSession.NextSeq(),
								Type: "response",
							},
							Message: "failed to disconnect: timeout",
							Command: dapRequest.Command,
						},
					}, nil
				}
			}

			return dap.DisconnectResponse{
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

func launchDebuggedProgram(programPath string, session *jsonrpc.Session, debugSession *DebugSession, fls *Filesystem) {
	sessionCtx := session.Context()

	defer func() {
		e := recover()
		switch val := e.(type) {
		case nil:
		case error:
			debugSession.programDoneChan <- val
		default:
			debugSession.programDoneChan <- fmt.Errorf("%#v: %s", val, string(debug.Stack()))
		}

		debugSession.finished.Store(true)

		session.Notify(jsonrpc.NotificationMessage{
			BaseMessage: jsonrpc.BaseMessage{
				Jsonrpc: JSONRPC_VERSION,
			},
			Method: "debug/terminatedEvent",
		})

		session.Notify(jsonrpc.NotificationMessage{
			BaseMessage: jsonrpc.BaseMessage{
				Jsonrpc: JSONRPC_VERSION,
			},
			Method: "debug/exitedEvent",
		})
	}()

	ctx := sessionCtx.BoundChildWithOptions(core.BoundChildContextOptions{
		Filesystem: fls,
	})

	programOut := utils.FnWriter{
		WriteFn: func(p []byte) (n int, err error) {
			outputEvent := dap.OutputEvent{
				Event: dap.Event{
					ProtocolMessage: dap.ProtocolMessage{
						Seq:  debugSession.NextSeq(),
						Type: "event",
					},
					Event: "output",
				},
				Body: dap.OutputEventBody{
					Output:   string(p),
					Category: "stdout",
				},
			}

			session.Notify(jsonrpc.NotificationMessage{
				BaseMessage: jsonrpc.BaseMessage{
					Jsonrpc: JSONRPC_VERSION,
				},
				Method: "debug/outputEvent",
				Params: utils.Must(json.Marshal(outputEvent)),
			})

			return len(p), nil
		},
	}

	debuggerOut := utils.FnWriter{
		WriteFn: func(p []byte) (n int, err error) {
			outputEvent := dap.OutputEvent{
				Event: dap.Event{
					ProtocolMessage: dap.ProtocolMessage{
						Seq:  debugSession.NextSeq(),
						Type: "event",
					},
					Event: "output",
				},
				Body: dap.OutputEventBody{
					Output:   string(p),
					Category: "console",
				},
			}

			session.Notify(jsonrpc.NotificationMessage{
				BaseMessage: jsonrpc.BaseMessage{
					Jsonrpc: JSONRPC_VERSION,
				},
				Method: "debug/outputEvent",
				Params: utils.Must(json.Marshal(outputEvent)),
			})

			return len(p), nil
		},
	}

	//create debugger

	var initialBreakpoints []core.BreakpointInfo
	debugSession.initialBreakpointsLock.Lock()
	for _, breakpoints := range debugSession.sourcePathToInitialBreakpoints {
		initialBreakpoints = append(initialBreakpoints, breakpoints...)
	}
	debugSession.sourcePathToInitialBreakpoints = nil
	debugSession.initialBreakpointsLock.Unlock()

	debugSession.debugger = core.NewDebugger(core.DebuggerArgs{
		Logger: zerolog.New(zerolog.NewConsoleWriter(func(w *zerolog.ConsoleWriter) {
			w.Out = debuggerOut
			w.NoColor = true
			w.PartsExclude = []string{zerolog.LevelFieldName}
			w.FieldsExclude = []string{"src"}
		})),
		InitialBreakpoints: initialBreakpoints,
	})

	//send a "stopped" event each time the program stops.
	go func() {
		stoppedChan := debugSession.debugger.StoppedChan()
		for {
			select {
			case stop, ok := <-stoppedChan:
				if !ok {
					return
				}

				stoppedEvent := dap.StoppedEvent{
					Event: dap.Event{
						ProtocolMessage: dap.ProtocolMessage{
							Seq:  debugSession.NextSeq(),
							Type: "event",
						},
						Event: "stopped",
					},
					Body: dap.StoppedEventBody{
						Reason:   stopReasonToDapStopReason(stop.Reason),
						ThreadId: 1,
					},
				}

				if stop.Breakpoint != nil {
					stoppedEvent.Body.HitBreakpointIds = []int{int(stop.Breakpoint.Id)}
				}

				session.Notify(jsonrpc.NotificationMessage{
					BaseMessage: jsonrpc.BaseMessage{
						Jsonrpc: JSONRPC_VERSION,
					},
					Method: "debug/stoppedEvent",
					Params: utils.Must(json.Marshal(stoppedEvent)),
				})
			case <-time.After(time.Second):
				if debugSession.debugger.Closed() {
					return
				}
			}
		}
	}()

	//send secondary events
	go func() {
		secondaryEventChan := debugSession.debugger.SecondaryEvents()
		for {
			select {
			case debugEvent, ok := <-secondaryEventChan:
				if !ok {
					return
				}

				eventType := debugEvent.SecondaryDebugEventType().String()
				//TODO: check format of event type

				dapEvent := DebugSecondaryEvent{
					Event: dap.Event{
						ProtocolMessage: dap.ProtocolMessage{
							Seq:  debugSession.NextSeq(),
							Type: "event",
						},
						Event: eventType,
					},
					Body: debugEvent,
				}

				session.Notify(jsonrpc.NotificationMessage{
					BaseMessage: jsonrpc.BaseMessage{
						Jsonrpc: JSONRPC_VERSION,
					},
					Method: "debug/" + eventType + "Event",
					Params: utils.Must(json.Marshal(dapEvent)),
				})
			case <-time.After(time.Second):
				if debugSession.debugger.Closed() {
					return
				}
			}
		}
	}()

	_, _, _, failedToPrepare, err := inox_ns.RunLocalScript(inox_ns.RunScriptArgs{
		Fpath:                     programPath,
		ParsingCompilationContext: ctx,
		ParentContext:             ctx,
		ParentContextRequired:     true,
		PreinitFilesystem:         fls,
		AllowMissingEnvVars:       false,
		IgnoreHighRiskScore:       true,
		Out:                       programOut,

		Debugger:     debugSession.debugger,
		PreparedChan: debugSession.programPreparedOrFailedToChan,
	})

	if !failedToPrepare {
		debugSession.programDoneChan <- err
	}
}

func stopReasonToDapStopReason(reason core.ProgramStopReason) string {
	switch reason {
	case core.PauseStop:
		return "pause"
	case core.StepStop:
		return "step"
	case core.BreakpointStop:
		return "breakpoint"
	default:
		panic(core.ErrUnreachable)
	}
}

func breakpointInfoToDebugAdapterProtocolBreakpoint(breakpoint core.BreakpointInfo, columnsStartAt1 bool) dap.Breakpoint {
	dapBreakpoint := dap.Breakpoint{
		Verified: breakpoint.Verified(),
		Id:       int(breakpoint.Id),
		Line:     int(breakpoint.StartLine),
		Column:   int(breakpoint.StartColumn),
	}

	if !columnsStartAt1 {
		dapBreakpoint.Column -= 1
	}

	src, ok := breakpoint.Chunk.Source.(parse.SourceFile)
	if ok && !src.IsResourceURL {
		dapBreakpoint.Source = &dap.Source{
			Name: src.Name(),
			Path: INOX_FS_SCHEME + "://" + src.Resource,
		}
	}

	return dapBreakpoint
}
