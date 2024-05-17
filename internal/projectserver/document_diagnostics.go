package projectserver

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/go-git/go-billy/v5/util"
	"github.com/inoxlang/inox/internal/codebase/analysis"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/hyperscript/hsanalysis"
	"github.com/inoxlang/inox/internal/hyperscript/hscode"
	"github.com/inoxlang/inox/internal/hyperscript/hsparse"
	"github.com/inoxlang/inox/internal/inoxconsts"
	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/project"
	"github.com/inoxlang/inox/internal/projectserver/jsonrpc"
	"github.com/inoxlang/inox/internal/sourcecode"
	"github.com/oklog/ulid/v2"

	"github.com/inoxlang/inox/internal/projectserver/lsp/defines"
	"github.com/inoxlang/inox/internal/utils"
)

const (
	//Duration before computing and publishing diagnostics after the user stops making edits.
	POST_EDIT_DIAGNOSTIC_DEBOUNCE_DURATION = 400 * time.Millisecond

	MIN_DURATION_BEFORE_LOW_PRIORITY_DOC_DIAG_RECOMPUTATION = time.Second
)

var (
	errSeverity     = defines.DiagnosticSeverityError
	warningSeverity = defines.DiagnosticSeverityWarning
)

// This handler does not return any diagnostics. Instead it spawns a goroutine that will compute, and push them using textDocument/publisDiagnostics.
// This is a bit of a hack, but unexpected bugs and issues arose when mixing the two diagnostic retrieval models (push and pull) was tried.
// Diagnostics may not be computed if a computation has happened very recently.
func handleDocumentDiagnostic(ctx context.Context, req *defines.DocumentDiagnosticParams) (any, error) {
	rpcSession := jsonrpc.GetSession(ctx)
	//sessionCtx := session.Context()

	//----------------------------------------------------------
	session := getCreateLockedProjectSession(rpcSession)
	projectMode := session.inProjectMode
	project := session.project
	fls := session.filesystem
	memberAuthToken := session.memberAuthToken
	lastCodebaseAnalysis := session.lastCodebaseAnalysis
	session.lock.Unlock()
	//----------------------------------------------------------

	if fls == nil {
		return &defines.FullDocumentDiagnosticReport{
			Kind:  defines.DocumentDiagnosticReportKindFull,
			Items: []defines.Diagnostic{},
		}, nil
	}

	uri := normalizeURI(req.TextDocument.Uri)
	_, err := getSupportedFilePath(uri, projectMode)
	if err != nil {
		return nil, err
	}

	go func() {
		defer utils.Recover()
		computeNotifyDocumentDiagnostics(diagnosticNotificationParams{
			triggeredByPull: true, //low priority
			docURI:          uri,
			usingInoxFS:     projectMode,

			rpcSession:           rpcSession,
			fls:                  fls,
			project:              project,
			memberAuthToken:      memberAuthToken,
			lastCodebaseAnalysis: lastCodebaseAnalysis,
		})
	}()

	// _ = fpath

	// // unchanged := defines.UnchangedDocumentDiagnosticReport{
	// // 	Kind:     defines.DocumentDiagnosticReportKindUnChanged,
	// // 	ResultId: "0",
	// // }

	// diagostics, err := computeDocumentDiagnostics(diagnosticNotificationParams{
	// 	session:         session,
	// 	docURI:          uri,
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
	rpcSession      *jsonrpc.Session
	docURI          defines.DocumentUri
	usingInoxFS     bool
	triggeredByPull bool

	fls                   *Filesystem
	project               *project.Project
	memberAuthToken       string
	inoxChunkCache        *parse.ChunkCache
	hyperscriptParseCache *hscode.ParseCache
	lastCodebaseAnalysis  *analysis.Result //may be nil
}

// computeNotifyDocumentDiagnostics diagnostics a document and notifies the LSP client (textDocument/publishDiagnostics).
// If $ignoreIfVeryRecentComputation is true and a call to computeNotifyDocumentDiagnostics has happened very recently the function does nothing.
func computeNotifyDocumentDiagnostics(params diagnosticNotificationParams) error {
	startTime := time.Now()

	projSession := getCreateLockedProjectSession(params.rpcSession)
	windowStartTimes := projSession.diagPullDisablingWindowStartTimes

	if params.triggeredByPull {
		timeSincePrevComputationStart := startTime.Sub(windowStartTimes[params.docURI])

		if timeSincePrevComputationStart < MIN_DURATION_BEFORE_LOW_PRIORITY_DOC_DIAG_RECOMPUTATION {
			projSession.lock.Unlock()
			return nil
		}
	}

	windowStartTimes[params.docURI] = startTime
	projSession.lock.Unlock()

	diagnostics, err := computeDocumentDiagnostics(params)
	if err != nil {
		return err
	}

	otherDocumentDiagnostics := diagnostics.otherDocumentDiagnostics
	//Note: the locking of $diagnostics is not necessary because otherDocumentDiagnostics are never updated.

	go func() {
		defer utils.Recover()
		for otherDocURI, otherDocDiagnostics := range otherDocumentDiagnostics {
			sendDocumentDiagnostics(params.rpcSession, otherDocURI, otherDocDiagnostics)
		}
	}()

	items := slices.Clone(diagnostics.items)

	if params.lastCodebaseAnalysis != nil {
		addErrorsAndWarningsAboutFileFromCodebaseAnalysis(params.lastCodebaseAnalysis, diagnostics.filePath, &items)
	}

	return sendDocumentDiagnostics(params.rpcSession, params.docURI, items)
}

// computes prepares a source file, constructs a list of defines.Diagnostic from errors at different phases
// (parsing, static check, and symbolic evaluation). The list is saved in the session before being returned.
func computeDocumentDiagnostics(params diagnosticNotificationParams) (result *singleDocumentDiagnostics, _ error) {

	docURI, usingInoxFS := params.docURI, params.usingInoxFS

	fpath, err := getSupportedFilePath(docURI, usingInoxFS)
	if err != nil {
		return nil, err
	}

	fileExtension := filepath.Ext(string(fpath))

	switch fileExtension {
	case inoxconsts.INOXLANG_FILE_EXTENSION:
		return computeInoxFileDiagnostics(params, fpath)
	case hscode.FILE_EXTENSION:
		return computeHyperscriptFileDiagnostics(params, fpath)
	default:
		return nil, errors.New("diagnostics can only be computed for Inox & Hyperscript files")
	}
}

func computeInoxFileDiagnostics(params diagnosticNotificationParams, fpath absoluteFilePath) (result *singleDocumentDiagnostics, _ error) {
	startTime := time.Now()
	fpathS := string(fpath)

	session, _, usingInoxFS, fls, project, memberAuthToken :=
		params.rpcSession, params.docURI, params.usingInoxFS, params.fls, params.project, params.memberAuthToken

	sessionCtx := session.Context()

	handlingCtx := sessionCtx.BoundChildWithOptions(core.BoundChildContextOptions{
		Filesystem: fls,
	})

	defer func() {
		if result != nil {
			result.finalize(fpath, startTime, params.rpcSession)
		}

		go func() {
			defer utils.Recover()
			handlingCtx.CancelGracefully()
		}()
	}()

	preparationResult, ok := prepareSourceFileInExtractionMode(handlingCtx, filePreparationParams{
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
	symbolicErrors := make(map[defines.DocumentUri][]symbolic.EvaluationError, 0)

	if !ok {
		return &singleDocumentDiagnostics{items: diagnostics}, nil
	}

	i := -1

	//Parsing diagnostics
	for _, err := range mod.Errors {
		i++

		pos := err.Position
		docURI, uriErr := getFileURI(pos.SourceName, usingInoxFS)
		text := err.BaseError.Error()

		//If the error is about the missing closing brace of a block we only show the rightmost
		//position in the error's range. Keeping the whole range would cause the editor to underline
		//all the block's range.
		if strings.Contains(text, parse.UNTERMINATED_BLOCK_MISSING_BRACE) {
			pos.StartLine = pos.EndLine
			pos.StartColumn = pos.EndColumn
			pos.Span.Start = pos.Span.End - 1
		}

		diagnostic := defines.Diagnostic{
			Message:  text,
			Severity: &errSeverity,
			Range:    rangeToLspRange(pos),
		}

		if pos.SourceName == fpathS {
			diagnostics = append(diagnostics, diagnostic)
		} else if uriErr == nil {
			otherDocumentDiagnostics[docURI] = append(otherDocumentDiagnostics[docURI], diagnostic)
		}
	}

	if state == nil {
		return &singleDocumentDiagnostics{items: diagnostics}, nil
	}

	//Add preinit static check errors.

	if state.PrenitStaticCheckErrors != nil {
		for _, err := range state.PrenitStaticCheckErrors {

			pos := getPositionInPositionStackOrFirst(err.Location, fpathS)
			docURI, uriErr := getFileURI(pos.SourceName, usingInoxFS)

			diagnostic := defines.Diagnostic{
				Message:  err.Message,
				Severity: &errSeverity,
				Range:    rangeToLspRange(pos),
			}

			if pos.SourceName == fpathS {
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
			pos = getPositionInPositionStackOrFirst(locatedEvalError.Location, fpathS)
			_range = rangeToLspRange(pos)
		} else {
			_range = firstCharsLspRange(5)
			msg = state.MainPreinitError.Error()
			pos = parse.SourcePositionRange{
				SourceName:  fpathS,
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

		if pos.SourceName == fpathS {
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

		for _, err := range state.StaticCheckData.Errors() {
			pos := getPositionInPositionStackOrFirst(err.Location, fpathS)
			docURI, uriErr := getFileURI(pos.SourceName, usingInoxFS)

			diagnostic := defines.Diagnostic{
				Message:  err.Message,
				Severity: &errSeverity,
				Range:    rangeToLspRange(pos),
			}

			if pos.SourceName == fpathS {
				diagnostics = append(diagnostics, diagnostic)
			} else if uriErr == nil {
				otherDocumentDiagnostics[docURI] = append(otherDocumentDiagnostics[docURI], diagnostic)
			}
		}

		//Add static check warnings.
		for _, warning := range state.StaticCheckData.Warnings() {
			pos := getPositionInPositionStackOrFirst(warning.Location, fpathS)
			docURI, uriErr := getFileURI(pos.SourceName, usingInoxFS)

			diagnostic := defines.Diagnostic{
				Message:  warning.Message,
				Severity: &warningSeverity,
				Range:    rangeToLspRange(pos),
			}

			if pos.SourceName == fpathS {
				diagnostics = append(diagnostics, diagnostic)
			} else if uriErr == nil {
				otherDocumentDiagnostics[docURI] = append(otherDocumentDiagnostics[docURI], diagnostic)
			}
		}

		//Add symbolic check errors.

		for _, err := range state.SymbolicData.Errors() {
			pos := getPositionInPositionStackOrFirst(err.Location, fpathS)
			docURI, uriErr := getFileURI(pos.SourceName, usingInoxFS)

			if uriErr == nil {
				symbolicErrors[docURI] = append(symbolicErrors[docURI], err)
			}

			diagnostic := defines.Diagnostic{
				Message:  err.Message,
				Severity: &errSeverity,
				Range:    rangeToLspRange(pos),
			}

			if pos.SourceName == fpathS {
				diagnostics = append(diagnostics, diagnostic)
			} else if uriErr == nil {
				otherDocumentDiagnostics[docURI] = append(otherDocumentDiagnostics[docURI], diagnostic)
			}
		}

		//Add symbolic check warnings.
		for _, warning := range state.SymbolicData.Warnings() {
			pos := getPositionInPositionStackOrFirst(warning.Location, fpathS)
			docURI, uriErr := getFileURI(pos.SourceName, usingInoxFS)

			diagnostic := defines.Diagnostic{
				Message:  warning.Message,
				Severity: &warningSeverity,
				Range:    rangeToLspRange(pos),
			}

			if pos.SourceName == fpathS {
				diagnostics = append(diagnostics, diagnostic)
			} else if uriErr == nil {
				otherDocumentDiagnostics[docURI] = append(otherDocumentDiagnostics[docURI], diagnostic)
			}
		}
	}

	return &singleDocumentDiagnostics{
		items:                    diagnostics,
		otherDocumentDiagnostics: otherDocumentDiagnostics,
		symbolicErrors:           symbolicErrors,
	}, nil
}

func computeHyperscriptFileDiagnostics(params diagnosticNotificationParams, fpath absoluteFilePath) (result *singleDocumentDiagnostics, _ error) {
	startTime := time.Now()
	fpathS := string(fpath)

	defer func() {
		if result != nil {
			result.finalize(fpath, startTime, params.rpcSession)
		}
	}()

	session, _, fls, hyperscriptFileCache :=
		params.rpcSession, params.docURI, params.fls, params.hyperscriptParseCache

	sessionCtx := session.Context()

	result = &singleDocumentDiagnostics{
		//we need the items list to be present in the notification so items should not be nil
		items: make([]defines.Diagnostic, 0),
	}

	//Parsing

	var sourceCode string
	{
		content, err := util.ReadFile(fls, fpathS)
		if err != nil {
			return
		}
		sourceCode = utils.BytesAsString(content) //after this line $content should not be used
	}

	sourceFile := sourcecode.File{
		NameString:             fpathS,
		UserFriendlyNameString: fpathS,
		Resource:               fpathS,
		ResourceDir:            filepath.Dir(fpathS),
		CodeString:             sourceCode,
	}
	parsedFile, criticalErr := hsparse.ParseFile(sessionCtx, sourceFile, hyperscriptFileCache, nil)

	if criticalErr != nil {
		return
	}

	if parsedFile.Error != nil {
		location := hscode.MakePositionFromParsingError(parsedFile.Error, fpathS)
		result.items = append(result.items, defines.Diagnostic{
			Range:    rangeToLspRange(location),
			Severity: &errSeverity,
			Message:  parsedFile.Error.Message,
		})
	}

	//Analysis

	if parsedFile.Result != nil {
		analysisResult, criticalErr := hsanalysis.Analyze(hsanalysis.Parameters{
			ProgramOrExpression: parsedFile.Result.NodeData,
			LocationKind:        hsanalysis.HyperscriptScriptFile,
			CodeStartIndex:      0,
			Chunk:               parsedFile,
		})
		if criticalErr != nil {
			return
		}

		for _, analysisErr := range analysisResult.Errors {
			result.items = append(result.items, makeDiagnosticFromLocatedError(analysisErr))
		}
		for _, warning := range analysisResult.Warnings {
			result.items = append(result.items, defines.Diagnostic{
				Range:    rangeToLspRange(warning.Location),
				Severity: &warningSeverity,
				Message:  warning.Message,
			})
		}
	}

	return
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
func MakeDocDiagnosticId(absPath absoluteFilePath) DocDiagnosticId {
	return DocDiagnosticId(ulid.Make().String() + "-" + string(absPath))
}

// A singleDocumentDiagnostics contains the diagnostics of a single Inox or Hyperscript document,
// it does not contain workspace diagnostics. This struct is never modified.
type singleDocumentDiagnostics struct {
	id        DocDiagnosticId
	filePath  absoluteFilePath
	startTime time.Time
	items     []defines.Diagnostic

	//Fields specific to Inox files.

	otherDocumentDiagnostics map[defines.DocumentUri][]defines.Diagnostic
	symbolicErrors           map[defines.DocumentUri][]symbolic.EvaluationError
}

func (d *singleDocumentDiagnostics) finalize(fpath absoluteFilePath, computeStart time.Time, rpcSession *jsonrpc.Session) {
	d.id = MakeDocDiagnosticId(fpath)
	d.filePath = fpath
	d.startTime = computeStart

	if d.items == nil {
		//Make sure the items are serialized as an empty array, not 'null'.
		d.items = []defines.Diagnostic{}
	}

	// Save $d in the session.
	projSession := getCreateLockedProjectSession(rpcSession)
	defer projSession.lock.Unlock()
	projSession.documentDiagnostics[fpath] = d
}
