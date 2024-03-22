package projectserver

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/project"
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
	rpcSession := jsonrpc.GetSession(ctx)
	//sessionCtx := session.Context()

	//----------------------------------------------------------
	session := getCreateLockedProjectSession(rpcSession)
	projectMode := session.inProjectMode
	project := session.project
	fls := session.filesystem
	memberAuthToken := session.memberAuthToken
	session.lock.Unlock()
	//----------------------------------------------------------

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
			docURI:      req.TextDocument.Uri,
			usingInoxFS: projectMode,

			rpcSession:      rpcSession,
			fls:             fls,
			project:         project,
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
		Kind: defines.DocumentDiagnosticReportKindFull,
		//Returning nil instead of []defines.Diagnostic{} causes VSCode to ignore the report.
		Items: []defines.Diagnostic{},
	}

	return report, nil
}

type diagnosticNotificationParams struct {
	rpcSession  *jsonrpc.Session
	docURI      defines.DocumentUri
	usingInoxFS bool

	fls             *Filesystem
	project         *project.Project
	memberAuthToken string
	inoxChunkCache  *parse.ChunkCache
}

// computeNotifyDocumentDiagnostics diagnostics a document and notifies the LSP client (textDocument/publishDiagnostics).
func computeNotifyDocumentDiagnostics(params diagnosticNotificationParams) error {
	diagnostics, err := computeDocumentDiagnostics(params)
	if err != nil {
		return err
	}

	go func() {
		defer utils.Recover()
		for otherDocURI, otherDocDiagnostics := range diagnostics.otherDocumentDiagnostics {
			sendDocumentDiagnostics(params.rpcSession, otherDocURI, otherDocDiagnostics)
		}
	}()

	return sendDocumentDiagnostics(params.rpcSession, params.docURI, diagnostics.items)
}

// computes prepares a source file, constructs a list of defines.Diagnostic from errors at different phases
// (parsing, static check, and symbolic evaluation). The list is saved in the session before being returned.
func computeDocumentDiagnostics(params diagnosticNotificationParams) (result *documentDiagnostics, _ error) {

	session, docURI, usingInoxFS, fls, project, memberAuthToken :=
		params.rpcSession, params.docURI, params.usingInoxFS, params.fls, params.project, params.memberAuthToken

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
			projSession := getCreateLockedProjectSession(params.rpcSession)
			defer projSession.lock.Unlock()
			projSession.documentDiagnostics[fpath] = result
		}
	}()

	errSeverity := defines.DiagnosticSeverityError
	warningSeverity := defines.DiagnosticSeverityWarning

	preparationResult, ok := prepareSourceFileInExtractionMode(ctx, filePreparationParams{
		fpath:         fpath,
		requiresState: false,
		ignoreCache:   true,

		rpcSession:      session,
		project:         project,
		lspFilesystem:   fls,
		inoxChunkCache:  params.inoxChunkCache,
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
	otherDocumentDiagnostics := map[defines.DocumentUri][]defines.Diagnostic{}

	if !ok {
		return &documentDiagnostics{items: diagnostics}, nil
	}

	i := -1

	//Parsing diagnostics
	for _, err := range mod.ParsingErrors {
		i++

		pos := mod.ParsingErrorPositions[i]
		docURI, uriErr := getFileURI(pos.SourceName, usingInoxFS)
		text := err.Text()

		//If the error is about the missing closing brace of a block we only show the rightmost
		//position in the error's range. Keeping the whole range would cause the editor to underline
		//all the block's range.
		if strings.Contains(text, parse.UNTERMINATED_BLOCK_MISSING_BRACE) {
			pos.StartLine = pos.EndLine
			pos.StartColumn = pos.EndColumn
			pos.Span.Start = pos.Span.End - 1
		}

		diagnostic := defines.Diagnostic{
			Message:  err.Text(),
			Severity: &errSeverity,
			Range:    rangeToLspRange(pos),
		}

		if pos.SourceName == fpath {
			diagnostics = append(diagnostics, diagnostic)
		} else if uriErr == nil {
			otherDocumentDiagnostics[docURI] = append(otherDocumentDiagnostics[docURI], diagnostic)
		}
	}

	if state == nil {
		return &documentDiagnostics{items: diagnostics}, nil
	}

	//Add preinit static check errors.

	if state.PrenitStaticCheckErrors != nil {
		for _, err := range state.PrenitStaticCheckErrors {

			pos := getPositionInPositionStackOrFirst(err.Location, fpath)
			docURI, uriErr := getFileURI(pos.SourceName, usingInoxFS)

			diagnostic := defines.Diagnostic{
				Message:  err.Message,
				Severity: &errSeverity,
				Range:    rangeToLspRange(pos),
			}

			if pos.SourceName == fpath {
				diagnostics = append(diagnostics, diagnostic)
			} else if uriErr == nil {
				otherDocumentDiagnostics[docURI] = append(otherDocumentDiagnostics[docURI], diagnostic)
			}
		}

	} else if state.MainPreinitError != nil {
		var _range defines.Range
		var msg string

		var (
			locatedEvalError core.LocatedEvalError
			pos              parse.SourcePositionRange
		)

		if errors.As(state.MainPreinitError, &locatedEvalError) {
			msg = locatedEvalError.Message
			pos = getPositionInPositionStackOrFirst(locatedEvalError.Location, fpath)
			_range = rangeToLspRange(pos)
		} else {
			_range = firstCharsLspRange(5)
			msg = state.MainPreinitError.Error()
			pos = parse.SourcePositionRange{
				SourceName:  fpath,
				StartLine:   1,
				StartColumn: 1,
				EndLine:     1,
				EndColumn:   1,
			}
		}

		docURI, uriErr := getFileURI(pos.SourceName, usingInoxFS)

		diagnostic := defines.Diagnostic{
			Message:  msg,
			Severity: &errSeverity,
			Range:    _range,
		}

		if pos.SourceName == fpath {
			diagnostics = append(diagnostics, diagnostic)
		} else if uriErr == nil {
			otherDocumentDiagnostics[docURI] = append(otherDocumentDiagnostics[docURI], diagnostic)
		}
	}

	if state.FirstDatabaseOpeningError != nil {
		session.Notify(NewShowMessage(defines.MessageTypeWarning, "failed to open at least one database: "+
			state.FirstDatabaseOpeningError.Error()))
	}

	if state.StaticCheckData != nil {
		//Add static check errors.

		for _, err := range state.SymbolicData.Errors() {
			pos := getPositionInPositionStackOrFirst(err.Location, fpath)
			docURI, uriErr := getFileURI(pos.SourceName, usingInoxFS)

			diagnostic := defines.Diagnostic{
				Message:  err.Message,
				Severity: &errSeverity,
				Range:    rangeToLspRange(pos),
			}

			if pos.SourceName == fpath {
				diagnostics = append(diagnostics, diagnostic)
			} else if uriErr == nil {
				otherDocumentDiagnostics[docURI] = append(otherDocumentDiagnostics[docURI], diagnostic)
			}
		}

		//Add static check warnings.
		for _, warning := range state.SymbolicData.Warnings() {
			pos := getPositionInPositionStackOrFirst(warning.Location, fpath)
			docURI, uriErr := getFileURI(pos.SourceName, usingInoxFS)

			diagnostic := defines.Diagnostic{
				Message:  warning.Message,
				Severity: &warningSeverity,
				Range:    rangeToLspRange(pos),
			}

			if pos.SourceName == fpath {
				diagnostics = append(diagnostics, diagnostic)
			} else if uriErr == nil {
				otherDocumentDiagnostics[docURI] = append(otherDocumentDiagnostics[docURI], diagnostic)
			}
		}

		//Add symbolic check errors.
		for _, err := range state.SymbolicData.Errors() {
			pos := getPositionInPositionStackOrFirst(err.Location, fpath)
			docURI, uriErr := getFileURI(pos.SourceName, usingInoxFS)

			diagnostic := defines.Diagnostic{
				Message:  err.Message,
				Severity: &errSeverity,
				Range:    rangeToLspRange(pos),
			}

			if pos.SourceName == fpath {
				diagnostics = append(diagnostics, diagnostic)
			} else if uriErr == nil {
				otherDocumentDiagnostics[docURI] = append(otherDocumentDiagnostics[docURI], diagnostic)
			}
		}

		//Add symbolic check warnings.
		for _, warning := range state.SymbolicData.Warnings() {
			pos := getPositionInPositionStackOrFirst(warning.Location, fpath)
			docURI, uriErr := getFileURI(pos.SourceName, usingInoxFS)

			diagnostic := defines.Diagnostic{
				Message:  warning.Message,
				Severity: &errSeverity,
				Range:    rangeToLspRange(pos),
			}

			if pos.SourceName == fpath {
				diagnostics = append(diagnostics, diagnostic)
			} else if uriErr == nil {
				otherDocumentDiagnostics[docURI] = append(otherDocumentDiagnostics[docURI], diagnostic)
			}
		}
	}

	return &documentDiagnostics{items: diagnostics, otherDocumentDiagnostics: otherDocumentDiagnostics}, nil
}

func sendDocumentDiagnostics(rpcSession *jsonrpc.Session, docURI defines.DocumentUri, diagnostics []defines.Diagnostic) error {

	version := int(
		time.Since(core.PROCESS_BEGIN_TIME) /
			/* Divide to prevent an overflow. A precision of 0.1 second should be fine. */
			(100 * time.Millisecond))

	return rpcSession.Notify(jsonrpc.NotificationMessage{
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
	otherDocumentDiagnostics     map[defines.DocumentUri][]defines.Diagnostic
	containsWorkspaceDiagnostics bool
	lock                         sync.Mutex
}
