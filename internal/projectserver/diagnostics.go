package projectserver

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/projectserver/jsonrpc"

	"github.com/inoxlang/inox/internal/projectserver/lsp/defines"
	"github.com/inoxlang/inox/internal/utils"
)

type diagnosticsParams struct {
	session         *jsonrpc.Session
	docURI          defines.DocumentUri
	usingInoxFS     bool
	fls             *Filesystem
	memberAuthToken string
}

// notifyDiagnostics prepares a source file, constructs a list of defines.Diagnostic from errors at different phases
// (parsing, static check, and symbolic evaluation) and notifies the LSP client (textDocument/publishDiagnostics).
func notifyDiagnostics(params diagnosticsParams) error {

	session, docURI, usingInoxFS, fls, memberAuthToken := params.session, params.docURI, params.usingInoxFS, params.fls, params.memberAuthToken

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

	preparationResult, ok := prepareSourceFileInExtractionMode(ctx, filePreparationParams{
		fpath:           fpath,
		session:         session,
		requiresState:   false,
		memberAuthToken: memberAuthToken,
	})

	state := preparationResult.state
	cachedOrGotCache := preparationResult.cachedOrGotCache
	mod := preparationResult.module

	if ok && !cachedOrGotCache && state != nil {
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
		sendDiagnostics(session, docURI, diagnostics)
		return nil
	}

	i := -1
	parsingDiagnostics := utils.MapSlice(mod.ParsingErrors, func(err core.Error) defines.Diagnostic {
		i++

		pos := mod.ParsingErrorPositions[i]
		text := err.Text()

		//If the error is about the missing closing brace of a block we only show the rightmost
		//position in the error's range. Keeping the whole range would cause the editor to underline
		//all the block's range.
		if strings.Contains(text, parse.UNTERMINATED_BLOCK_MISSING_BRACE) {
			pos.StartLine = pos.EndLine
			pos.StartColumn = pos.EndColumn
			pos.Span.Start = pos.Span.End - 1
		}

		return defines.Diagnostic{
			Message:  err.Text(),
			Severity: &errSeverity,
			Range:    rangeToLspRange(pos),
		}
	})

	diagnostics = append(diagnostics, parsingDiagnostics...)

	if state == nil {
		sendDiagnostics(session, docURI, diagnostics)
		return nil
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
		//Add static check errors.
		i := -1
		staticCheckErrorDiagnostics := utils.MapSlice(state.StaticCheckData.Errors(), func(err *core.StaticCheckError) defines.Diagnostic {
			i++

			return defines.Diagnostic{
				Message:  err.Message,
				Severity: &errSeverity,
				Range:    rangeToLspRange(getPositionInPositionStackOrFirst(err.Location, fpath)),
			}
		})
		diagnostics = append(diagnostics, staticCheckErrorDiagnostics...)

		//Add static check warnings.
		i = -1
		staticCheckWarningDiagnostics := utils.MapSlice(state.StaticCheckData.Warnings(), func(warning *core.StaticCheckWarning) defines.Diagnostic {
			i++

			return defines.Diagnostic{
				Message:  warning.Message,
				Severity: &warningSeverity,
				Range:    rangeToLspRange(getPositionInPositionStackOrFirst(warning.Location, fpath)),
			}
		})

		diagnostics = append(diagnostics, staticCheckWarningDiagnostics...)

		//Add symbolic check errors.
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

		//Add symbolic check warnings.
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

	sendDiagnostics(session, docURI, diagnostics)
	return nil
}

func sendDiagnostics(session *jsonrpc.Session, docURI defines.DocumentUri, diagnostics []defines.Diagnostic) error {
	return session.Notify(jsonrpc.NotificationMessage{
		Method: "textDocument/publishDiagnostics",
		Params: utils.Must(json.Marshal(defines.PublishDiagnosticsParams{
			Uri:         docURI,
			Diagnostics: diagnostics,
		})),
	})
}
