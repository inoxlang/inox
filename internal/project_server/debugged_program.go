package project_server

import (
	"fmt"
	"runtime/debug"
	"time"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/http_ns"
	"github.com/inoxlang/inox/internal/hack"
	"github.com/inoxlang/inox/internal/mod"
	"github.com/inoxlang/inox/internal/project_server/jsonrpc"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/rs/zerolog"
)

type debuggedProgramLaunch struct {
	programPath  string
	logLevels    *core.LogLevels
	session      *jsonrpc.Session
	debugSession *DebugSession
	fls          *Filesystem
}

func launchDebuggedProgram(args debuggedProgramLaunch) {
	programPath := args.programPath
	logLevels := args.logLevels
	session := args.session
	sessionCtx := session.Context()
	debugSession := args.debugSession
	fls := args.fls

	defer func() {
		e := recover()

		var err error
		switch val := e.(type) {
		case nil:
		case error:
			err = fmt.Errorf("%w: %s", val, string(debug.Stack()))
			debugSession.programDoneChan <- err
		default:
			err = fmt.Errorf("%#v: %s", val, string(debug.Stack()))
			debugSession.programDoneChan <- err
		}

		debugSession.finished.Store(true)

		session.Notify(jsonrpc.NotificationMessage{
			Method: "debug/terminatedEvent",
		})

		session.Notify(jsonrpc.NotificationMessage{
			Method: "debug/exitedEvent",
		})
	}()

	ctx := sessionCtx.BoundChildWithOptions(core.BoundChildContextOptions{
		Filesystem: fls,
	})

	project, _ := getProject(session)

	programOut := utils.FnWriter{
		WriteFn: func(p []byte) (n int, err error) {
			notifyOutputEvent(string(p), "stdout", debugSession, session)
			return len(p), nil
		},
	}

	debuggerOut := utils.FnWriter{
		WriteFn: func(p []byte) (n int, err error) {
			notifyOutputEvent(string(p), "console", debugSession, session)
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
	exceptionBreakpointsId := debugSession.initialExceptionBreakpointsId
	debugSession.initialBreakpointsLock.Unlock()

	debugSession.debugger = core.NewDebugger(core.DebuggerArgs{
		Logger: zerolog.New(zerolog.NewConsoleWriter(func(w *zerolog.ConsoleWriter) {
			w.Out = debuggerOut
			w.NoColor = true
			w.PartsExclude = []string{zerolog.LevelFieldName}
			w.FieldsExclude = []string{"src"}
		})),
		InitialBreakpoints:    initialBreakpoints,
		ExceptionBreakpointId: exceptionBreakpointsId,
	})
	debugSession.debuggerSet.Store(true)

	//create logger with the configured log level

	logger := zerolog.New(zerolog.NewConsoleWriter(func(w *zerolog.ConsoleWriter) {
		w.Out = programOut
		w.NoColor = false
		w.TimeFormat = time.TimeOnly
	}))

	logger = logger.Hook(zerolog.HookFunc(func(e *zerolog.Event, level zerolog.Level, message string) {
		//discard log events from http/certmagic.

		if level > zerolog.DebugLevel {
			return
		}

		s, ok := hack.GetLogEventStringFieldValue(e, core.QUOTED_SOURCE_LOG_FIELD_NAME)
		if !ok {
			return
		}

		if s == http_ns.CERT_MAGIG_LOG_SRC {
			e.Discard()
		}
	}))

	_, _, _, preparationOk, err := mod.RunLocalScript(mod.RunScriptArgs{
		Fpath:                     programPath,
		ParsingCompilationContext: ctx,
		ParentContext:             ctx,
		ParentContextRequired:     true,
		PreinitFilesystem:         fls,
		AllowMissingEnvVars:       false,
		IgnoreHighRiskScore:       true,
		FullAccessToDatabases:     true,
		Project:                   project,

		Out:       programOut,
		Logger:    logger,
		LogLevels: logLevels,

		Debugger:     debugSession.debugger,
		PreparedChan: debugSession.programPreparedOrFailedToChan,
	})

	if preparationOk {
		debugSession.programDoneChan <- err
	} else {
		debugSession.debugger.Closed()
	}
}
