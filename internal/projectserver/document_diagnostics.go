package projectserver

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/projectserver/jsonrpc"
	"github.com/oklog/ulid/v2"

	"github.com/inoxlang/inox/internal/projectserver/lsp/defines"
	"github.com/inoxlang/inox/internal/utils"
)

const (
	//Duration before computing and publishing diagnostics after the user stops making edits.
	POST_EDIT_DIAGNOSTIC_DEBOUNCE_DURATION = 400 * time.Millisecond
)

// This handler does not return any diagnostics. Instead it spawns a goroutine that will compute, and push them using textDocument/publisDiagnostics.
// This is a bit of a hack, but unexpected bugs and issues arose when mixing the two diagnostic retrieval models (push and pull) was tried.
func handleDocumentDiagnostic(ctx context.Context, req *defines.DocumentDiagnosticParams) (any, error) {
	session := jsonrpc.GetSession(ctx)
	//sessionCtx := session.Context()

	sessionData := getLockedSessionData(session)
	projectMode := sessionData.projectMode
	fls := sessionData.filesystem
	memberAuthToken := sessionData.memberAuthToken
	sessionData.lock.Unlock()

	if fls == nil {
		return &defines.FullDocumentDiagnosticReport{
			Kind:  defines.DocumentDiagnosticReportKindFull,
			Items: []defines.Diagnostic{},
		}, nil
	}

	_, err := getFilePath(req.TextDocument.Uri, projectMode)
	if err != nil {
		return nil, err
	}

	go func() {
		defer utils.Recover()
		computeNotifyDocumentDiagnostics(diagnosticNotificationParams{
			session:         session,
			docURI:          req.TextDocument.Uri,
			usingInoxFS:     projectMode,
			fls:             fls,
			memberAuthToken: memberAuthToken,
		})
	}()

	// _ = fpath

	// // unchanged := defines.UnchangedDocumentDiagnosticReport{
	// // 	Kind:     defines.DocumentDiagnosticReportKindUnChanged,
	// // 	ResultId: "0",
	// // }

	// diagostics, err := computeDocumentDiagnostics(diagnosticNotificationParams{
	// 	session:         session,
	// 	docURI:          req.TextDocument.Uri,
	// 	usingInoxFS:     projectMode,
	// 	fls:             fls,
	// 	memberAuthToken: memberAuthToken,
	// })

	// if err != nil {
	// 	return nil, jsonrpc.ResponseError{
	// 		Code:    jsonrpc.InternalError.Code,
	// 		Message: err.Error(),
	// 	}
	// }

	report := &defines.FullDocumentDiagnosticReport{
		Kind:  defines.DocumentDiagnosticReportKindFull,
		Items: []defines.Diagnostic{},
	}

	return report, nil
}

type diagnosticNotificationParams struct {
	session         *jsonrpc.Session
	docURI          defines.DocumentUri
	usingInoxFS     bool
	fls             *Filesystem
	memberAuthToken string
}

// computeNotifyDocumentDiagnostics diagnostics a document and notifies the LSP client (textDocument/publishDiagnostics).
func computeNotifyDocumentDiagnostics(params diagnosticNotificationParams) error {
	diagnostics, err := computeDocumentDiagnostics(params)
	if err != nil {
		return err
	}
	return sendDocumentDiagnostics(params.session, params.docURI, diagnostics.items)
}

// computes prepares a source file, constructs a list of defines.Diagnostic from errors at different phases
// (parsing, static check, and symbolic evaluation). The list is saved in the session before being returned.
func computeDocumentDiagnostics(params diagnosticNotificationParams) (result *documentDiagnostics, _ error) {

	session, docURI, usingInoxFS, fls, memberAuthToken := params.session, params.docURI, params.usingInoxFS, params.fls, params.memberAuthToken

	sessionCtx := session.Context()
	ctx := sessionCtx.BoundChildWithOptions(core.BoundChildContextOptions{
		Filesystem: fls,
	})

	fpath, err := getFilePath(docURI, usingInoxFS)
	if err != nil {
		return nil, err
	}

	defer func() {
		if result != nil {
			//Finalize the result.
			result.id = MakeDocDiagnosticId(fpath)
			if result.items == nil {
				//Make sure the items are serialized as an empty array, not 'null'.
				result.items = []defines.Diagnostic{}
			}

			// //Save the result in the session.
			projSession := getLockedSessionData(params.session)
			defer projSession.lock.Unlock()
			projSession.documentDiagnostics[fpath] = result
		}
	}()

	errSeverity := defines.DiagnosticSeverityError
	warningSeverity := defines.DiagnosticSeverityWarning

	preparationResult, ok := prepareSourceFileInExtractionMode(ctx, filePreparationParams{
		fpath:           fpath,
		session:         session,
		requiresState:   false,
		memberAuthToken: memberAuthToken,
		ignoreCache:     true,
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
		return &documentDiagnostics{items: diagnostics}, nil
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
		return &documentDiagnostics{items: diagnostics}, nil
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

	return &documentDiagnostics{items: diagnostics}, nil
}

func sendDocumentDiagnostics(session *jsonrpc.Session, docURI defines.DocumentUri, diagnostics []defines.Diagnostic) error {

	version := int(
		time.Since(core.PROCESS_BEGIN_TIME) /
			/* Divide to prevent an overflow. A precision of 0.1 second should be fine. */
			(100 * time.Millisecond))

	return session.Notify(jsonrpc.NotificationMessage{
		Method: "textDocument/publishDiagnostics",
		Params: utils.Must(json.Marshal(defines.PublishDiagnosticsParams{
			Uri:         docURI,
			Diagnostics: diagnostics,
			//Setting a version seens to make VSCode more likely to override old pulled diagnostics with published ones.
			Version: &version,
		})),
	})
}

// Format: <ULID>-absolute-document-path
// Example: 01HRTBRGXEWG6T4M6N4V4QVP0F-/main.ix
type DocDiagnosticId string

// MakeDocDiagnosticId returns a DocDiagnosticId for the document at $absPath.
// The time of the ULID part is the current time.
func MakeDocDiagnosticId(absPath string) DocDiagnosticId {
	return DocDiagnosticId(ulid.Make().String() + "-" + absPath)
}

type documentDiagnostics struct {
	id                           DocDiagnosticId
	items                        []defines.Diagnostic
	containsWorkspaceDiagnostics bool
	lock                         sync.Mutex
}
