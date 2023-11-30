package project_server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"runtime/debug"
	"strings"
	"time"

	"github.com/google/go-dap"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/project_server/jsonrpc"
	"github.com/inoxlang/inox/internal/project_server/logs"
	"github.com/inoxlang/inox/internal/project_server/lsp"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/rs/zerolog"
)

const (
	DEFAULT_DEBUG_COMMAND_TIMEOUT = 2 * time.Second
	EXCEPTION_ERROR_FILTER        = "exception"

	DEFAULT_MAX_SESSION_COUNT = 2
	DEFAULT_LOG_LEVEL         = zerolog.DebugLevel
)

var (
	ErrUnknowSessionId                = errors.New("unknown session id")
	ErrSessionAlreadyExists           = errors.New("session already exists")
	ErrMaxParallelDebugSessionReached = errors.New("the maximum number of parallel debug sessions is already reached")
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

type DebugNextParams struct {
	SessionId string              `json:"sessionID"`
	Request   dap.ContinueRequest `json:"request"`
}

type DebugStepInParams struct {
	SessionId string              `json:"sessionID"`
	Request   dap.ContinueRequest `json:"request"`
}

type DebugStepOutParams struct {
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

type DebugSetExceptionBreakpointsParams struct {
	SessionId string                             `json:"sessionID"`
	Request   dap.SetExceptionBreakpointsRequest `json:"request"`
}

type DebugLaunchArgs struct {
	Program  string                                     `json:"program"`
	LogLevel map[ /*'default' or a path*/ string]string `json:"logLevel,omitempty"`
}

type DebugDisconnectParams struct {
	SessionId string                `json:"sessionID"`
	Request   dap.DisconnectRequest `json:"request"`
}

type DebugSecondaryEvent struct {
	dap.Event
	Body any `json:"body"`
}

func registerDebugMethodHandlers(
	server *lsp.Server, opts LSPServerConfiguration,
) {

	getDebugSession := func(session *jsonrpc.Session, sessionId string) (*DebugSession, error) {
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

	createDebugSession := func(session *jsonrpc.Session, sessionId string) (*DebugSession, error) {
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

			variablesReferences: make(map[core.StateId]*variablesReferences, 0),
		}
		debugSession.nextSeq.Store(1)
		debugSessions.AddSession(debugSession)

		return debugSession, nil
	}

	removeDebugSession := func(debugSession *DebugSession, session *jsonrpc.Session) {
		sessionData := getLockedSessionData(session)
		debugSessions := sessionData.debugSessions
		sessionData.lock.Unlock()
		if debugSessions != nil {
			debugSessions.RemoveSession(debugSession)
		}
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

			debugSession, err := createDebugSession(session, params.SessionId)
			if err != nil {
				return dap.InitializeResponse{
					Response: dap.Response{
						RequestSeq: dapRequest.Seq,
						Success:    false,
						ProtocolMessage: dap.ProtocolMessage{
							Seq:  0,
							Type: "response",
						},
						Command: dapRequest.Command,
						Message: err.Error(),
					},
				}, nil
			}

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
					SupportsConfigurationDoneRequest:      true,
					SupportsSingleThreadExecutionRequests: true,
					ExceptionBreakpointFilters: []dap.ExceptionBreakpointsFilter{
						{
							Filter: EXCEPTION_ERROR_FILTER,
							Label:  "Exception Errors",
						},
					},
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

			debugSession, err := getDebugSession(session, params.SessionId)

			if err != nil {
				return dap.ConfigurationDoneResponse{
					Response: dap.Response{
						RequestSeq: dapRequest.Seq,
						Success:    false,
						ProtocolMessage: dap.ProtocolMessage{
							Seq:  0,
							Type: "response",
						},
						Command: dapRequest.Command,
						Message: err.Error(),
					},
				}, nil
			}

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

			debugSession, err := getDebugSession(session, params.SessionId)

			makeDAPErrorResponse := func(msg string) dap.LaunchResponse {
				return dap.LaunchResponse{
					Response: dap.Response{
						RequestSeq: dapRequest.Seq,
						Success:    false,
						ProtocolMessage: dap.ProtocolMessage{
							Seq:  debugSession.NextSeq(),
							Type: "response",
						},
						Command: dapRequest.Command,
						Message: msg,
					},
				}
			}

			if err != nil {
				return makeDAPErrorResponse(err.Error()), nil
			}

			fls, ok := getLspFilesystem(session)
			if !ok {
				removeDebugSession(debugSession, session)
				return nil, errors.New(string(FsNoFilesystem))
			}

			//check the configuration is done

			if !debugSession.configurationDone.Load() {
				return makeDAPErrorResponse("failed to launch: configuration is not done"), nil
			}

			//check the program is not already running

			if debugSession.programDoneChan != nil {
				return makeDAPErrorResponse("program is already running"), nil
			}

			//unmarshal user arguments

			var launchArgs DebugLaunchArgs
			err = json.Unmarshal(([]byte(dapRequest.Arguments)), &launchArgs)
			if err != nil {
				removeDebugSession(debugSession, session)

				return nil, jsonrpc.ResponseError{
					Code:    jsonrpc.InternalError.Code,
					Message: err.Error(),
				}
			}

			//check user arguments

			if launchArgs.Program == "" {
				removeDebugSession(debugSession, session)
				return makeDAPErrorResponse("missing program in launch arguments"), nil
			}

			var defaultLogLevel zerolog.Level = DEFAULT_LOG_LEVEL
			logLevelByPath := map[core.Path]zerolog.Level{}
			enableInternalDebugLogs := false

			if launchArgs.LogLevel != nil {
				defaultLevel, ok := launchArgs.LogLevel["default"]
				if !ok {
					removeDebugSession(debugSession, session)
					return makeDAPErrorResponse("missing default log level"), nil
				}

				defaultLogLevel, err = zerolog.ParseLevel(defaultLevel)

				if err != nil {
					removeDebugSession(debugSession, session)
					return makeDAPErrorResponse(fmt.Sprintf("invalid default log level: %q", defaultLevel)), nil
				}

				for key, level := range launchArgs.LogLevel {
					if key == "" || key[0] != '/' {
						continue
					}

					parsedLogLevel, err := zerolog.ParseLevel(level)

					if err != nil {
						removeDebugSession(debugSession, session)
						return makeDAPErrorResponse(fmt.Sprintf("invalid default log level: %q", level)), nil
					}
					logLevelByPath[core.NonDirPathFrom(key)] = parsedLogLevel
				}
			}

			// update the debug session

			logs.Println("program: ", launchArgs.Program)
			programPath := filepath.Clean(launchArgs.Program)

			debugSession.programPath = programPath
			debugSession.programDoneChan = make(chan error, 1)
			debugSession.programPreparedOrFailedToChan = make(chan error)

			//teardown goroutine
			go func() {
				defer func() {
					if e := recover(); e != nil {
						err := utils.ConvertPanicValueToError(e)
						logs.Println(fmt.Errorf("%w: %s", err, string(debug.Stack())))
					}
				}()

				defer removeDebugSession(debugSession, session)

				select {
				case <-session.Context().Done():
					return
				case err := <-debugSession.programDoneChan:
					if err != nil {
						notifyOutputEvent("program failed: "+err.Error(), "important", debugSession, session)
					}
				}
			}()


			go launchDebuggedProgram(debuggedProgramLaunch{
				programPath:  programPath,
				logLevels:    core.NewLogLevels(defaultLogLevel, logLevelByPath, enableInternalDebugLogs),
				session:      session,
				debugSession: debugSession,
				fls:          fls,
			})

			err = <-debugSession.programPreparedOrFailedToChan
			if err != nil {
				removeDebugSession(debugSession, session)

				return makeDAPErrorResponse("program: " + err.Error()), nil
			}

			startDebugEventSenders(debugSession, session)

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

			debugSession, err := getDebugSession(session, params.SessionId)

			if err != nil {
				return dap.ThreadsResponse{
					Response: dap.Response{
						RequestSeq: dapRequest.Seq,
						Success:    false,
						ProtocolMessage: dap.ProtocolMessage{
							Seq:  0,
							Type: "response",
						},
						Command: dapRequest.Command,
						Message: err.Error(),
					},
				}, nil
			}

			var threads []dap.Thread

			if !utils.InefficientlyWaitUntilTrue(&debugSession.debuggerSet, 10*time.Millisecond) {
				return dap.ThreadsResponse{
					Response: dap.Response{
						RequestSeq: dapRequest.Seq,
						Success:    false,
						ProtocolMessage: dap.ProtocolMessage{
							Seq:  0,
							Type: "response",
						},
						Command: dapRequest.Command,
						Message: "debuggee",
					},
				}, nil
			}

			for _, thread := range debugSession.debugger.Threads() {
				threads = append(threads, dap.Thread{
					Id:   int(thread.Id),
					Name: thread.Name,
				})
			}

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

			debugSession, err := getDebugSession(session, params.SessionId)

			if err != nil {
				return dap.StackTraceResponse{
					Response: dap.Response{
						RequestSeq: dapRequest.Seq,
						Success:    false,
						ProtocolMessage: dap.ProtocolMessage{
							Seq:  0,
							Type: "response",
						},
						Command: dapRequest.Command,
						Message: err.Error(),
					},
				}, nil
			}

			var stackFrames []dap.StackFrame
			framesChan := make(chan []dap.StackFrame)

			debugSession.debugger.ControlChan() <- core.DebugCommandGetStackTrace{
				ThreadId: core.StateId(params.Request.Arguments.ThreadId),
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
				stackFrames = stackFrames[:min(len(stackFrames), maxFrames)]
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

			debugSession, err := getDebugSession(session, params.SessionId)

			if err != nil {
				return dap.ScopesResponse{
					Response: dap.Response{
						RequestSeq: dapRequest.Seq,
						Success:    false,
						ProtocolMessage: dap.ProtocolMessage{
							Seq:  0,
							Type: "response",
						},
						Command: dapRequest.Command,
						Message: err.Error(),
					},
				}, nil
			}

			var scopes []dap.Scope
			scopesChan := make(chan []dap.Scope)

			threadId, ok := debugSession.debugger.ThreadIfOfStackFrame(int32(params.Request.Arguments.FrameId))

			if !ok {
				return dap.ScopesResponse{
					Response: dap.Response{
						RequestSeq: dapRequest.Seq,
						Success:    false,
						ProtocolMessage: dap.ProtocolMessage{
							Seq:  debugSession.NextSeq(),
							Type: "response",
						},
						Message: "failed to get scopes: failed to find thread of stack frame",
						Command: dapRequest.Command,
					},
				}, nil
			}

			debugSession.debugger.ControlChan() <- core.DebugCommandGetScopes{
				ThreadId: threadId,
				Get: func(globalScope, localScope map[string]core.Value) {

					refs := debugSession.getThreadVariablesReferences(threadId)

					scopesChan <- []dap.Scope{
						{
							Name:               "Local Scope",
							PresentationHint:   "locals",
							NamedVariables:     len(localScope),
							VariablesReference: refs.localScope,
						},
						{
							Name:               "Global Scope",
							NamedVariables:     len(globalScope),
							VariablesReference: refs.globalScope,
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

			debugSession, err := getDebugSession(session, params.SessionId)

			if err != nil {
				return dap.VariablesResponse{
					Response: dap.Response{
						RequestSeq: dapRequest.Seq,
						Success:    false,
						ProtocolMessage: dap.ProtocolMessage{
							Seq:  0,
							Type: "response",
						},
						Command: dapRequest.Command,
						Message: err.Error(),
					},
				}, nil
			}

			var variables []dap.Variable
			varsChan := make(chan []dap.Variable)

			ref := dapRequest.Arguments.VariablesReference
			threadId, refs, ok := debugSession.getThreadOfVariablesReference(params.Request.Arguments.VariablesReference)

			if !ok {
				return dap.VariablesResponse{
					Response: dap.Response{
						RequestSeq: dapRequest.Seq,
						Success:    false,
						ProtocolMessage: dap.ProtocolMessage{
							Seq:  debugSession.NextSeq(),
							Type: "response",
						},
						Message: "failed to get variables: failed to find thread",
						Command: dapRequest.Command,
					},
				}, nil
			}

			debugSession.debugger.ControlChan() <- core.DebugCommandGetScopes{
				ThreadId: threadId,
				Get: func(globalScope, localScope map[string]core.Value) {
					var variables []dap.Variable

					handlingCtx := session.Context().BoundChild()

					var scope map[string]core.Value

					switch ref {
					case refs.globalScope:
						scope = globalScope
					case refs.localScope:
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

			debugSession, err := getDebugSession(session, params.SessionId)

			if err != nil {
				return dap.SetBreakpointsResponse{
					Response: dap.Response{
						RequestSeq: dapRequest.Seq,
						Success:    false,
						ProtocolMessage: dap.ProtocolMessage{
							Seq:  0,
							Type: "response",
						},
						Command: dapRequest.Command,
						Message: err.Error(),
					},
				}, nil
			}

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
				return nil, errors.New(string(FsNoFilesystem))
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
		Name: "debug/setExceptionBreakpoints",
		NewRequest: func() interface{} {
			return &DebugSetExceptionBreakpointsParams{}
		},
		Handler: func(ctx context.Context, req interface{}) (interface{}, error) {
			session := jsonrpc.GetSession(ctx)
			params := req.(*DebugSetExceptionBreakpointsParams)
			dapRequest := params.Request

			debugSession, err := getDebugSession(session, params.SessionId)

			if err != nil {
				return dap.SetExceptionBreakpointsResponse{
					Response: dap.Response{
						RequestSeq: dapRequest.Seq,
						Success:    false,
						ProtocolMessage: dap.ProtocolMessage{
							Seq:  0,
							Type: "response",
						},
						Command: dapRequest.Command,
						Message: err.Error(),
					},
				}, nil
			}

			//initial exception breakpoints
			if !debugSession.configurationDone.Load() {
				debugSession.initialBreakpointsLock.Lock()
				id := debugSession.nextInitialBreakpointId
				debugSession.nextInitialBreakpointId++

				debugSession.initialExceptionBreakpointsId = id
				debugSession.initialBreakpointsLock.Unlock()

				return dap.SetExceptionBreakpointsResponse{
					Response: dap.Response{
						RequestSeq: dapRequest.Seq,
						Success:    true,
						ProtocolMessage: dap.ProtocolMessage{
							Seq:  debugSession.NextSeq(),
							Type: "response",
						},
						Command: dapRequest.Command,
					},
					Body: dap.SetExceptionBreakpointsResponseBody{
						Breakpoints: []dap.Breakpoint{
							{
								Id: int(id),
							},
						},
					},
				}, nil
			}

			//else non-initial exception breakpoints (program is launched)

			disable := true
			for _, filter := range dapRequest.Arguments.Filters {
				if filter == EXCEPTION_ERROR_FILTER {
					disable = false
					break
				}
			}

			idChan := make(chan int32)

			cmd := core.DebugCommandSetExceptionBreakpoints{
				Disable: disable,
				GetExceptionBreakpointId: func(i int32) {
					idChan <- i
				},
			}

			debugSession.debugger.ControlChan() <- cmd

			var id int32

			select {
			case id = <-idChan:
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

			return dap.SetExceptionBreakpointsResponse{
				Response: dap.Response{
					RequestSeq: dapRequest.Seq,
					Success:    true,
					ProtocolMessage: dap.ProtocolMessage{
						Seq:  debugSession.NextSeq(),
						Type: "response",
					},
					Command: dapRequest.Command,
				},
				Body: dap.SetExceptionBreakpointsResponseBody{
					Breakpoints: []dap.Breakpoint{
						{
							Id:       int(id),
							Verified: true,
						},
					},
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

			debugSession, err := getDebugSession(session, params.SessionId)

			if err != nil {
				return dap.PauseResponse{
					Response: dap.Response{
						RequestSeq: dapRequest.Seq,
						Success:    false,
						ProtocolMessage: dap.ProtocolMessage{
							Seq:  0,
							Type: "response",
						},
						Command: dapRequest.Command,
						Message: err.Error(),
					},
				}, nil
			}

			debugger := debugSession.debugger
			if !debugger.Closed() {
				debugger.ControlChan() <- core.DebugCommandPause{
					ThreadId: core.StateId(params.Request.Arguments.ThreadId),
				}
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

			debugSession, err := getDebugSession(session, params.SessionId)

			if err != nil {
				return dap.ContinueResponse{
					Response: dap.Response{
						RequestSeq: dapRequest.Seq,
						Success:    false,
						ProtocolMessage: dap.ProtocolMessage{
							Seq:  0,
							Type: "response",
						},
						Command: dapRequest.Command,
						Message: err.Error(),
					},
				}, nil
			}

			debugger := debugSession.debugger
			if !debugger.Closed() {
				//TODO: support continuing a specific thread (see params)
				debugger.ControlChan() <- core.DebugCommandContinue{
					ThreadId:         core.StateId(params.Request.Arguments.ThreadId),
					ResumeAllThreads: !params.Request.Arguments.SingleThread,
				}
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
		Name: "debug/next",
		NewRequest: func() interface{} {
			return &DebugNextParams{}
		},
		Handler: func(ctx context.Context, req interface{}) (interface{}, error) {
			session := jsonrpc.GetSession(ctx)
			params := req.(*DebugNextParams)
			dapRequest := params.Request

			debugSession, err := getDebugSession(session, params.SessionId)

			if err != nil {
				return dap.NextResponse{
					Response: dap.Response{
						RequestSeq: dapRequest.Seq,
						Success:    false,
						ProtocolMessage: dap.ProtocolMessage{
							Seq:  0,
							Type: "response",
						},
						Command: dapRequest.Command,
						Message: err.Error(),
					},
				}, nil
			}

			debugger := debugSession.debugger
			if !debugger.Closed() {
				debugger.ControlChan() <- core.DebugCommandNextStep{
					ThreadId:         core.StateId(params.Request.Arguments.ThreadId),
					ResumeAllThreads: !params.Request.Arguments.SingleThread,
				}
			}

			return dap.NextResponse{
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
		Name: "debug/stepIn",
		NewRequest: func() interface{} {
			return &DebugStepInParams{}
		},
		Handler: func(ctx context.Context, req interface{}) (interface{}, error) {
			session := jsonrpc.GetSession(ctx)
			params := req.(*DebugStepInParams)
			dapRequest := params.Request

			debugSession, err := getDebugSession(session, params.SessionId)

			if err != nil {
				return dap.StepInResponse{
					Response: dap.Response{
						RequestSeq: dapRequest.Seq,
						Success:    false,
						ProtocolMessage: dap.ProtocolMessage{
							Seq:  0,
							Type: "response",
						},
						Command: dapRequest.Command,
						Message: err.Error(),
					},
				}, nil
			}

			debugger := debugSession.debugger
			if !debugger.Closed() {
				debugger.ControlChan() <- core.DebugCommandStepIn{
					ThreadId:         core.StateId(params.Request.Arguments.ThreadId),
					ResumeAllThreads: !params.Request.Arguments.SingleThread,
				}
			}

			return dap.StepInResponse{
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
		Name: "debug/stepOut",
		NewRequest: func() interface{} {
			return &DebugStepOutParams{}
		},
		Handler: func(ctx context.Context, req interface{}) (interface{}, error) {
			session := jsonrpc.GetSession(ctx)
			params := req.(*DebugStepOutParams)
			dapRequest := params.Request

			debugSession, err := getDebugSession(session, params.SessionId)

			if err != nil {
				return dap.StepOutResponse{
					Response: dap.Response{
						RequestSeq: dapRequest.Seq,
						Success:    false,
						ProtocolMessage: dap.ProtocolMessage{
							Seq:  0,
							Type: "response",
						},
						Command: dapRequest.Command,
						Message: err.Error(),
					},
				}, nil
			}

			debugger := debugSession.debugger
			if !debugger.Closed() {
				debugger.ControlChan() <- core.DebugCommandStepOut{
					ThreadId:         core.StateId(params.Request.Arguments.ThreadId),
					ResumeAllThreads: !params.Request.Arguments.SingleThread,
				}
			}

			return dap.StepOutResponse{
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
		Name: "debug/disconnect",
		NewRequest: func() interface{} {
			return &DebugDisconnectParams{}
		},
		Handler: func(ctx context.Context, req interface{}) (interface{}, error) {
			session := jsonrpc.GetSession(ctx)
			params := req.(*DebugDisconnectParams)
			dapRequest := params.Request

			debugSession, err := getDebugSession(session, params.SessionId)
			if err != nil {
				return dap.DisconnectResponse{
					Response: dap.Response{
						RequestSeq: dapRequest.Seq,
						Success:    false,
						ProtocolMessage: dap.ProtocolMessage{
							Seq:  0,
							Type: "response",
						},
						Command: dapRequest.Command,
						Message: err.Error(),
					},
				}, nil
			}

			debugger := debugSession.debugger

			doneChan := make(chan struct{})

			if debugger != nil && !debugger.Closed() {
				debugger.ControlChan() <- core.DebugCommandCloseDebugger{
					CancelExecution: !debugSession.wasAttached,
					Done: func() {
						doneChan <- struct{}{}
					},
				}

				defer removeDebugSession(debugSession, session)

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

func stopReasonToDapStopReason(reason core.ProgramStopReason) string {
	switch reason {
	case core.PauseStop:
		return "pause"
	case core.NextStepStop, core.StepInStop, core.StepOutStop:
		return "step"
	case core.BreakpointStop:
		return "breakpoint"
	case core.ExceptionBreakpointStop:
		return "exception"
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

func notifyOutputEvent(msg string, category string, debugSession *DebugSession, session *jsonrpc.Session) {
	outputEvent := dap.OutputEvent{
		Event: dap.Event{
			ProtocolMessage: dap.ProtocolMessage{
				Seq:  debugSession.NextSeq(),
				Type: "event",
			},
			Event: "output",
		},
		Body: dap.OutputEventBody{
			Output:   msg,
			Category: category,
		},
	}

	session.Notify(jsonrpc.NotificationMessage{
		Method: "debug/outputEvent",
		Params: utils.Must(json.Marshal(outputEvent)),
	})
}

func startDebugEventSenders(debugSession *DebugSession, session *jsonrpc.Session) {
	//goroutine sending a "stopped" event each time the program stops.
	go func() {
		defer func() {
			if e := recover(); e != nil {
				err := utils.ConvertPanicValueToError(e)
				logs.Println(fmt.Errorf("%w: %s", err, string(debug.Stack())))
			}
		}()

		stoppedChan := debugSession.debugger.StoppedChan()
		for {
			select {
			case stop, ok := <-stoppedChan:
				if !ok {
					return
				}

				body := dap.StoppedEventBody{
					Reason:   stopReasonToDapStopReason(stop.Reason),
					ThreadId: int(stop.ThreadId),
				}

				if stop.ExceptionError != nil {
					//TODO: make sure no sensitive information is leaked
					body.Description = stop.ExceptionError.Error()
					body.Text = body.Description
				}

				stoppedEvent := dap.StoppedEvent{
					Event: dap.Event{
						ProtocolMessage: dap.ProtocolMessage{
							Seq:  debugSession.NextSeq(),
							Type: "event",
						},
						Event: "stopped",
					},
					Body: body,
				}

				if stop.Breakpoint != nil {
					stoppedEvent.Body.HitBreakpointIds = []int{int(stop.Breakpoint.Id)}
				}

				session.Notify(jsonrpc.NotificationMessage{
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

	//goroutine sending secondary events
	go func() {
		defer func() {
			if e := recover(); e != nil {
				err := utils.ConvertPanicValueToError(e)
				logs.Println(fmt.Errorf("%w: %s", err, string(debug.Stack())))
			}
		}()

		secondaryEventChan := debugSession.debugger.SecondaryEvents()
		for {
			select {
			case debugEvent, ok := <-secondaryEventChan:
				if !ok {
					return
				}

				eventType := debugEvent.SecondaryDebugEventType().String()

				commonEventData := dap.Event{
					ProtocolMessage: dap.ProtocolMessage{
						Seq:  debugSession.NextSeq(),
						Type: "event",
					},
					Event: eventType,
				}

				//handle some events separately
				switch e := debugEvent.(type) {
				case core.LThreadSpawnedEvent:
					session.Notify(jsonrpc.NotificationMessage{
						Method: "debug/threadEvent",
						Params: utils.Must(json.Marshal(dap.ThreadEvent{
							Event: commonEventData,
							Body: dap.ThreadEventBody{
								Reason:   "started",
								ThreadId: int(e.StateId),
							},
						})),
					})
					continue
				}

				//TODO: check format of event type

				dapEvent := DebugSecondaryEvent{
					Event: commonEventData,
					Body:  debugEvent,
				}

				session.Notify(jsonrpc.NotificationMessage{
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

}
