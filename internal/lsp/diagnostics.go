package internal

import (
	"encoding/json"
	"errors"
	"io"

	core "github.com/inoxlang/inox/internal/core"
	symbolic "github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/globals/inox_ns"

	"github.com/inoxlang/inox/internal/lsp/jsonrpc"
	"github.com/inoxlang/inox/internal/lsp/logs"
	"github.com/inoxlang/inox/internal/lsp/lsp/defines"
	"github.com/inoxlang/inox/internal/utils"
)

func notifyDiagnostics(session *jsonrpc.Session, docURI defines.DocumentUri, usingInoxFS bool) error {
	sessionCtx := session.Context()
	fls := sessionCtx.GetFileSystem()

	fpath, err := getFilePath(docURI, usingInoxFS)
	if err != nil {
		return err
	}

	errSeverity := defines.DiagnosticSeverityError
	warningSeverity := defines.DiagnosticSeverityWarning

	state, mod, _, err := inox_ns.PrepareLocalScript(inox_ns.ScriptPreparationArgs{
		Fpath:                     fpath,
		ParsingCompilationContext: sessionCtx,
		ParentContext:             nil,
		Out:                       io.Discard,
		DevMode:                   true,
		AllowMissingEnvVars:       true,
		ScriptContextFileSystem:   fls,
		PreinitFilesystem:         fls,
	})

	if mod == nil { //unrecoverable parsing error
		session.Notify(NewShowMessage(defines.MessageTypeError, "failed to prepare script: "+err.Error()))
		return nil
	}

	//we need the diagnostics list to be present in the notification so diagnostics should not be nil
	diagnostics := make([]defines.Diagnostic, 0)

	if err == nil {
		logs.Println("no errors")
		goto send_diagnostics
	}

	if err != nil && state == nil && mod == nil {
		logs.Println("err", err)
		goto send_diagnostics
	}

	{

		i := -1
		parsingDiagnostics := utils.MapSlice(mod.ParsingErrors, func(err core.Error) defines.Diagnostic {
			i++

			return defines.Diagnostic{
				Message:  err.Text(),
				Severity: &errSeverity,
				Range:    rangeToLspRange(mod.ParsingErrorPositions[i]),
			}
		})

		diagnostics = append(diagnostics, parsingDiagnostics...)

		if state == nil {
			goto send_diagnostics
		}

		if state.PrenitStaticCheckErrors != nil {
			i := -1
			staticCheckDiagnostics := utils.MapSlice(state.PrenitStaticCheckErrors, func(err *core.StaticCheckError) defines.Diagnostic {
				i++

				return defines.Diagnostic{
					Message:  err.Message,
					Severity: &errSeverity,
					Range:    rangeToLspRange(err.Location[0]),
				}
			})

			diagnostics = append(diagnostics, staticCheckDiagnostics...)
		}

		if state.MainPreinitError != nil {
			var _range defines.Range
			var msg string

			var locatedEvalError core.LocatedEvalError

			if errors.As(state.MainPreinitError, &locatedEvalError) {
				msg = locatedEvalError.Message
				_range = rangeToLspRange(locatedEvalError.Location[0])
			} else {
				_range = firstCharsLspRange(5)
				msg = state.MainPreinitError.Error()
			}

			diagnostics = append(diagnostics, defines.Diagnostic{
				Message:  msg,
				Severity: &errSeverity,
				Range:    _range,
			})
		}

		if state.FirstDatabaseOpeningError != nil {
			session.Notify(NewShowMessage(defines.MessageTypeWarning, "failed to open at least one database: "+err.Error()))
		}

		if state.StaticCheckData != nil {
			i := -1
			staticCheckDiagnostics := utils.MapSlice(state.StaticCheckData.Errors(), func(err *core.StaticCheckError) defines.Diagnostic {
				i++

				return defines.Diagnostic{
					Message:  err.Message,
					Severity: &errSeverity,
					Range:    rangeToLspRange(err.Location[0]),
				}
			})

			diagnostics = append(diagnostics, staticCheckDiagnostics...)

			i = -1
			symbolicCheckErrorDiagnostics := utils.MapSlice(state.SymbolicData.Errors(), func(err symbolic.SymbolicEvaluationError) defines.Diagnostic {
				i++

				return defines.Diagnostic{
					Message:  err.Message,
					Severity: &errSeverity,
					Range:    rangeToLspRange(err.Location[0]),
				}
			})

			diagnostics = append(diagnostics, symbolicCheckErrorDiagnostics...)

			symbolicCheckWarningDiagnostics := utils.MapSlice(state.SymbolicData.Warnings(), func(err symbolic.SymbolicEvaluationWarning) defines.Diagnostic {
				i++

				return defines.Diagnostic{
					Message:  err.Message,
					Severity: &warningSeverity,
					Range:    rangeToLspRange(err.Location[0]),
				}
			})

			diagnostics = append(diagnostics, symbolicCheckWarningDiagnostics...)
		}
	}

send_diagnostics:
	session.Notify(jsonrpc.NotificationMessage{
		BaseMessage: jsonrpc.BaseMessage{
			Jsonrpc: JSONRPC_VERSION,
		},
		Method: "textDocument/publishDiagnostics",
		Params: utils.Must(json.Marshal(defines.PublishDiagnosticsParams{
			Uri:         docURI,
			Diagnostics: diagnostics,
		})),
	})

	return nil
}
