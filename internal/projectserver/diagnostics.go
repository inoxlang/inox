package projectserver

import (
	"encoding/json"
	"errors"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/projectserver/jsonrpc"

	"github.com/inoxlang/inox/internal/projectserver/lsp/defines"
	"github.com/inoxlang/inox/internal/utils"
)

func notifyDiagnostics(session *jsonrpc.Session, docURI defines.DocumentUri, usingInoxFS bool, fls *Filesystem) error {
	sessionCtx := session.Context()
	ctx := sessionCtx.BoundChildWithOptions(core.BoundChildContextOptions{
		Filesystem: fls,
	})

	fpath, err := getFilePath(docURI, usingInoxFS)
	if err != nil {
		return err
	}

	errSeverity := defines.DiagnosticSeverityError
	warningSeverity := defines.DiagnosticSeverityWarning

	state, mod, _, cachedOrGotCache, ok := prepareSourceFileInExtractionMode(ctx, filePreparationParams{
		fpath:         fpath,
		session:       session,
		requiresState: false,
	})
	if !cachedOrGotCache && state != nil {
		//teardown in separate goroutine to return quickly
		defer func() {
			if state != nil {
				go func() {
					defer utils.Recover()
					state.Ctx.CancelGracefully()
				}()
			}
		}()
	}
	//a context cancellations is not deferred because

	//we need the diagnostics list to be present in the notification so diagnostics should not be nil
	diagnostics := make([]defines.Diagnostic, 0)

	if !ok {
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
					Range:    rangeToLspRange(getPositionInPositionStackOrFirst(err.Location, fpath)),
				}
			})

			diagnostics = append(diagnostics, staticCheckDiagnostics...)
		} else if state.MainPreinitError != nil {
			var _range defines.Range
			var msg string

			var locatedEvalError core.LocatedEvalError

			if errors.As(state.MainPreinitError, &locatedEvalError) {
				msg = locatedEvalError.Message
				_range = rangeToLspRange(getPositionInPositionStackOrFirst(locatedEvalError.Location, fpath))
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
			session.Notify(NewShowMessage(defines.MessageTypeWarning, "failed to open at least one database: "+
				state.FirstDatabaseOpeningError.Error()))
		}

		if state.StaticCheckData != nil {
			i := -1
			staticCheckDiagnostics := utils.MapSlice(state.StaticCheckData.Errors(), func(err *core.StaticCheckError) defines.Diagnostic {
				i++

				return defines.Diagnostic{
					Message:  err.Message,
					Severity: &errSeverity,
					Range:    rangeToLspRange(getPositionInPositionStackOrFirst(err.Location, fpath)),
				}
			})

			diagnostics = append(diagnostics, staticCheckDiagnostics...)

			i = -1
			symbolicCheckErrorDiagnostics := utils.MapSlice(state.SymbolicData.Errors(), func(err symbolic.SymbolicEvaluationError) defines.Diagnostic {
				i++

				return defines.Diagnostic{
					Message:  err.Message,
					Severity: &errSeverity,
					Range:    rangeToLspRange(getPositionInPositionStackOrFirst(err.Location, fpath)),
				}
			})

			diagnostics = append(diagnostics, symbolicCheckErrorDiagnostics...)

			symbolicCheckWarningDiagnostics := utils.MapSlice(state.SymbolicData.Warnings(), func(err symbolic.SymbolicEvaluationWarning) defines.Diagnostic {
				i++

				return defines.Diagnostic{
					Message:  err.Message,
					Severity: &warningSeverity,
					Range:    rangeToLspRange(getPositionInPositionStackOrFirst(err.Location, fpath)),
				}
			})

			diagnostics = append(diagnostics, symbolicCheckWarningDiagnostics...)
		}
	}

send_diagnostics:
	session.Notify(jsonrpc.NotificationMessage{
		Method: "textDocument/publishDiagnostics",
		Params: utils.Must(json.Marshal(defines.PublishDiagnosticsParams{
			Uri:         docURI,
			Diagnostics: diagnostics,
		})),
	})

	return nil
}
