package projectserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	fsutil "github.com/go-git/go-billy/v5/util"
	"github.com/google/uuid"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/inoxconsts"
	"github.com/inoxlang/inox/internal/projectserver/jsonrpc"
	"github.com/inoxlang/inox/internal/projectserver/logs"
	"github.com/inoxlang/inox/internal/projectserver/lsp"

	"github.com/inoxlang/inox/internal/projectserver/lsp/defines"

	"github.com/inoxlang/inox/internal/utils"

	"net/url"

	"github.com/inoxlang/inox/internal/parse"
)

const (
	INOX_FS_SCHEME = "inox"
)

var (
	sessionToAdditionalData     = make(map[*jsonrpc.Session]*additionalSessionData)
	sessionToAdditionalDataLock sync.Mutex

	ErrFileURIExpected        = errors.New("a file: URI was expected")
	ErrInoxURIExpected        = errors.New("a inox: URI was expected")
	ErrMemberNotAuthenticated = errors.New("member not authenticated")

	True  = true
	False = false
)

func registerStandardMethodHandlers(server *lsp.Server, serverConfig LSPServerConfiguration) {
	projectMode := serverConfig.ProjectMode

	//Session initialization and shutdown

	server.OnInitialize(func(ctx context.Context, req *defines.InitializeParams) (result *defines.InitializeResult, err *defines.InitializeError) {
		return handleInitialize(ctx, req, projectMode)
	})

	server.OnShutdown(handleShutdown)

	server.OnExit(handleExit)

	//Intellisense

	server.OnHover(handleHover)

	server.OnSignatureHelp(handleSignatureHelp)

	server.OnCompletion(handleCompletion)

	server.OnCodeActionWithSliceCodeAction(handleCodeActionWithSliceCodeAction)

	server.OnDefinition(handleDefinition)

	//Document synchronization

	server.OnDidOpenTextDocument(handleDidOpenDocument)

	server.OnDidSaveTextDocument(handleDidSaveDocument)

	server.OnDidChangeTextDocument(handleDidChangeDocument)

	server.OnDidCloseTextDocument(handleDidCloseDocument)

	//Formatting

	server.OnDocumentFormatting(handleFormatDocument)

}

func getFilePath(uri defines.DocumentUri, usingInoxFs bool) (string, error) {
	u, err := url.Parse(string(uri))
	if err != nil {
		return "", fmt.Errorf("invalid URI: %s: %w", uri, err)
	}
	if usingInoxFs && u.Scheme != INOX_FS_SCHEME {
		return "", fmt.Errorf("%w, URI is: %s", ErrInoxURIExpected, string(uri))
	}
	if !usingInoxFs && u.Scheme != "file" {
		return "", fmt.Errorf("%w, URI is: %s", ErrFileURIExpected, string(uri))
	}

	clean := filepath.Clean(u.Path)
	if !strings.HasSuffix(clean, inoxconsts.INOXLANG_FILE_EXTENSION) {
		return "", fmt.Errorf("unxepected file extension: '%s'", filepath.Ext(clean))
	}
	return clean, nil
}

func getFileURI(path string, usingInoxFs bool) (defines.DocumentUri, error) {
	if path == "" {
		return "", errors.New("failed to get document URI: empty path")
	}
	if path[0] != '/' {
		return "", fmt.Errorf("failed to get document URI: path is not absolute: %q", path)
	}
	if usingInoxFs {
		return defines.DocumentUri(INOX_FS_SCHEME + "://" + path), nil
	}
	return defines.DocumentUri("file://" + path), nil
}

func getPath(uri defines.URI, usingInoxFS bool) (string, error) {
	u, err := url.Parse(string(uri))
	if err != nil {
		return "", fmt.Errorf("invalid URI: %s: %w", uri, err)
	}
	if usingInoxFS && u.Scheme != INOX_FS_SCHEME {
		return "", fmt.Errorf("%w, actual is: %s", ErrInoxURIExpected, string(uri))
	}
	if !usingInoxFS && u.Scheme != "file" {
		return "", fmt.Errorf("%w, actual is: %s", ErrFileURIExpected, string(uri))
	}
	return filepath.Clean(u.Path), nil
}

func handleInitialize(ctx context.Context, req *defines.InitializeParams, projectMode bool) (result *defines.InitializeResult, err *defines.InitializeError) {
	session := jsonrpc.GetSession(ctx)
	s := &defines.InitializeResult{}

	s.Capabilities.HoverProvider = true
	s.Capabilities.SignatureHelpProvider = &defines.SignatureHelpOptions{
		TriggerCharacters:   &[]string{"(", ","},
		RetriggerCharacters: &[]string{","},
	}
	s.Capabilities.WorkspaceSymbolProvider = true
	s.Capabilities.DefinitionProvider = true
	s.Capabilities.CodeActionProvider = &defines.CodeActionOptions{
		CodeActionKinds: &[]defines.CodeActionKind{defines.CodeActionKindQuickFix},
	}
	s.Capabilities.DocumentFormattingProvider = true

	if *req.Capabilities.TextDocument.Synchronization.DidSave && *req.Capabilities.TextDocument.Synchronization.DynamicRegistration {
		s.Capabilities.TextDocumentSync = defines.TextDocumentSyncKindIncremental
	} else {
		s.Capabilities.TextDocumentSync = defines.TextDocumentSyncKindFull
	}

	s.Capabilities.CompletionProvider = &defines.CompletionOptions{
		TriggerCharacters: &[]string{".", ":", "{", "-", "/"},
	}

	//create session data
	{
		sessionData := getLockedSessionData(session)
		sessionData.clientCapabilities = req.Capabilities
		sessionData.serverCapabilities = s.Capabilities
		sessionData.projectMode = projectMode
		sessionData.lock.Unlock()
	}

	//remove closed sessions
	sessionToAdditionalDataLock.Lock()
	for s, data := range sessionToAdditionalData {
		sessionToRemove := s
		if sessionToRemove.Closed() {
			logs.Println("remove one closed session: " + s.Client())
			delete(sessionToAdditionalData, sessionToRemove)
			func() {
				data.lock.Lock()
				defer data.lock.Unlock()
				if data.preparedSourceFilesCache != nil {
					data.preparedSourceFilesCache.acknowledgeSessionEnd()
					data.preparedSourceFilesCache = nil
				}
			}()
		}
	}
	newCount := len(sessionToAdditionalData)
	sessionToAdditionalDataLock.Unlock()
	logs.Println("current session count:", newCount)

	// remove session and data on shutdown or when closed
	var removed atomic.Bool
	removeSession := func(s *jsonrpc.Session) {
		if !removed.CompareAndSwap(false, true) {
			return
		}
		logs.Println("remove one session that has just finished: " + s.Client())
		sessionToAdditionalDataLock.Lock()
		sessionData, ok := sessionToAdditionalData[s]
		delete(sessionToAdditionalData, s)
		sessionToAdditionalDataLock.Unlock()

		if ok {
			func() {
				sessionData.lock.Lock()
				defer sessionData.lock.Unlock()
				sessionData.preparedSourceFilesCache.acknowledgeSessionEnd()
				sessionData.preparedSourceFilesCache = nil

				if sessionData.serverAPI != nil {
					sessionData.serverAPI.acknowledgeSessionEnd()
				}
			}()
		}
	}

	session.SetShutdownCallbackFn(removeSession)
	session.SetClosedCallbackFn(removeSession)

	return s, nil
}

func handleShutdown(ctx context.Context, req *defines.NoParams) (err error) {
	session := jsonrpc.GetSession(ctx)
	session.Close()
	return nil
}

func handleExit(ctx context.Context, req *defines.NoParams) (err error) {
	session := jsonrpc.GetSession(ctx)
	defer session.Close()

	if !session.IsShuttingDown() {
		return errors.New("the client should make a shutdown request before sending an exit notification")
	}

	return nil
}

func handleHover(ctx context.Context, req *defines.HoverParams) (result *defines.Hover, err error) {
	session := jsonrpc.GetSession(ctx)
	sessionCtx := session.Context()

	sessionData := getLockedSessionData(session)
	projectMode := sessionData.projectMode
	fls := sessionData.filesystem
	memberAuthToken := sessionData.memberAuthToken
	lastCodebaseAnalysis := sessionData.lastCodebaseAnalysis
	sessionData.lock.Unlock()

	if fls == nil {
		return nil, errors.New(string(FsNoFilesystem))
	}

	fpath, err := getFilePath(req.TextDocument.Uri, projectMode)
	if err != nil {
		return nil, err
	}
	line, column := getLineColumn(req.Position)

	handlingCtx := sessionCtx.BoundChildWithOptions(core.BoundChildContextOptions{
		Filesystem: fls,
	})
	defer handlingCtx.CancelGracefully()

	return getHoverContent(handlingCtx, hoverContentParams{
		fpath:                fpath,
		line:                 line,
		column:               column,
		session:              session,
		memberAuthToken:      memberAuthToken,
		lastCodebaseAnalysis: lastCodebaseAnalysis,
	})
}

func handleSignatureHelp(ctx context.Context, req *defines.SignatureHelpParams) (result *defines.SignatureHelp, err error) {
	session := jsonrpc.GetSession(ctx)
	sessionCtx := session.Context()

	sessionData := getLockedSessionData(session)
	projectMode := sessionData.projectMode
	fls := sessionData.filesystem
	memberAuthToken := sessionData.memberAuthToken
	sessionData.lock.Unlock()

	if fls == nil {
		return nil, errors.New(string(FsNoFilesystem))
	}

	if memberAuthToken == "" {
		return nil, ErrMemberNotAuthenticated
	}

	fpath, err := getFilePath(req.TextDocument.Uri, projectMode)
	if err != nil {
		return nil, err
	}
	line, column := getLineColumn(req.Position)

	handlingCtx := sessionCtx.BoundChildWithOptions(core.BoundChildContextOptions{
		Filesystem: fls,
	})
	defer handlingCtx.CancelGracefully()

	return getSignatureHelp(handlingCtx, signatureHelpParams{
		fpath:           fpath,
		line:            line,
		column:          column,
		session:         session,
		memberAuthToken: memberAuthToken,
	})
}

func handleCodeActionWithSliceCodeAction(ctx context.Context, req *defines.CodeActionParams) (result *[]defines.CodeAction, err error) {
	session := jsonrpc.GetSession(ctx)
	sessionData := getLockedSessionData(session)
	projectMode := sessionData.projectMode
	fls := sessionData.filesystem
	sessionData.lock.Unlock()

	if fls == nil {
		return nil, nil
	}

	fpath, err := getFilePath(req.TextDocument.Uri, projectMode)
	if err != nil {
		return nil, err
	}

	actions, err := getCodeActions(session, req.Context.Diagnostics, req.Range, req.TextDocument, fpath, fls)

	if err != nil {
		logs.Println("failed to get code actions", err)
		return nil, nil
	}
	return actions, nil
}

func handleDefinition(ctx context.Context, req *defines.DefinitionParams) (result *[]defines.LocationLink, err error) {
	session := jsonrpc.GetSession(ctx)
	sessionCtx := session.Context()

	sessionData := getLockedSessionData(session)
	projectMode := sessionData.projectMode
	fls := sessionData.filesystem
	memberAuthToken := sessionData.memberAuthToken
	sessionData.lock.Unlock()

	if fls == nil {
		return nil, errors.New(string(FsNoFilesystem))
	}

	fpath, err := getFilePath(req.TextDocument.Uri, projectMode)
	if err != nil {
		return nil, err
	}
	line, column := getLineColumn(req.Position)

	handlingCtx := sessionCtx.BoundChildWithOptions(core.BoundChildContextOptions{
		Filesystem: fls,
	})
	defer handlingCtx.CancelGracefully()

	preparationResult, ok := prepareSourceFileInExtractionMode(handlingCtx, filePreparationParams{
		fpath:           fpath,
		session:         session,
		requiresState:   true,
		memberAuthToken: memberAuthToken,
	})

	state := preparationResult.state
	chunk := preparationResult.chunk
	cachedOrGotCache := preparationResult.cachedOrGotCache

	if !cachedOrGotCache && state != nil {
		//teardown in separate goroutine to return quickly
		defer func() {
			go func() {
				defer utils.Recover()
				state.Ctx.CancelGracefully()
			}()
		}()
	}

	if !ok || state == nil || state.SymbolicData == nil {
		logs.Println("failed to prepare source file", err)
		return nil, nil
	}

	span := chunk.GetLineColumnSingeCharSpan(line, column)
	foundNode, ancestors, ok := chunk.GetNodeAndChainAtSpan(span)

	if !ok || foundNode == nil {
		logs.Println("no data: node not found")
		return nil, nil
	}

	var position parse.SourcePositionRange
	positionSet := false

	switch n := foundNode.(type) {
	case *parse.Variable, *parse.GlobalVariable, *parse.IdentifierLiteral:
		position, positionSet = state.SymbolicData.GetVariableDefinitionPosition(foundNode, ancestors)
	case *parse.PatternIdentifierLiteral, *parse.PatternNamespaceIdentifierLiteral:
		position, positionSet = state.SymbolicData.GetNamedPatternOrPatternNamespacePositionDefinition(foundNode, ancestors)
	case *parse.RelativePathLiteral, *parse.AbsolutePathLiteral:

		parent := ancestors[len(ancestors)-1]

		switch p := parent.(type) {
		case *parse.InclusionImportStatement, *parse.ImportStatement:

			file, isFile := chunk.Source.(parse.SourceFile)
			if !isFile || file.IsResourceURL || file.ResourceDir == "" {
				break
			}

			path := n.(parse.SimpleValueLiteral).ValueString()
			if path[0] != '/' { //relative
				path = filepath.Join(file.ResourceDir, path)
			}

			position = parse.SourcePositionRange{
				SourceName:  path,
				StartLine:   1,
				StartColumn: 1,
				EndLine:     1,
				EndColumn:   2,
				Span:        parse.NodeSpan{Start: 0, End: 1},
			}
			positionSet = true
		case *parse.ObjectProperty:
			absPathLit, ok := n.(*parse.AbsolutePathLiteral)

			if !ok || len(ancestors) < 4 || p.HasImplicitKey() || p.Name() != "databases" {
				break
			}

			_, ok = ancestors[len(ancestors)-2].(*parse.ObjectLiteral)
			if !ok {
				break
			}
			_, ok = ancestors[len(ancestors)-3].(*parse.Manifest)
			if !ok {
				break
			}

			position = parse.SourcePositionRange{
				SourceName:  absPathLit.Value,
				StartLine:   1,
				StartColumn: 1,
				EndLine:     1,
				EndColumn:   2,
				Span:        parse.NodeSpan{Start: 0, End: 1},
			}
			positionSet = true
		}
	}

	if !positionSet {
		logs.Println("no data")
		return nil, nil
	}

	links := []defines.LocationLink{
		{
			TargetUri:            defines.DocumentUri(sessionData.Scheme() + "://" + position.SourceName),
			TargetRange:          rangeToLspRange(position),
			TargetSelectionRange: rangeToLspRange(position),
		},
	}
	return &links, nil
}

func handleFormatDocument(ctx context.Context, req *defines.DocumentFormattingParams) (result *[]defines.TextEdit, err error) {
	session := jsonrpc.GetSession(ctx)

	//----------------------------------------
	sessionData := getLockedSessionData(session)
	projectMode := sessionData.projectMode
	fls := sessionData.filesystem
	sessionData.lock.Unlock()
	//----------------------------------------

	if fls == nil {
		return nil, nil
	}

	fpath, err := getFilePath(req.TextDocument.Uri, projectMode)
	if err != nil {
		return nil, err
	}

	chunk, err := core.ParseFileChunk(fpath, fls, parse.ParserOptions{
		Timeout: SINGLE_FILE_PARSING_TIMEOUT,
	})
	if chunk == nil { //unrecoverable error
		return nil, jsonrpc.ResponseError{
			Code:    jsonrpc.InternalError.Code,
			Message: fmt.Sprintf("failed to parse file: %s", err),
		}
	}

	formatted := formatInoxChunk(chunk, req.Options)
	fullRange := rangeToLspRange(chunk.GetSourcePosition(chunk.Node.Span))

	return &[]defines.TextEdit{
		{
			Range:   fullRange,
			NewText: formatted,
		},
	}, nil
}

func handleDidOpenDocument(ctx context.Context, req *defines.DidOpenTextDocumentParams) (err error) {
	session := jsonrpc.GetSession(ctx)

	//----------------------------------------
	sessionData := getLockedSessionData(session)
	projectMode := sessionData.projectMode
	fls := sessionData.filesystem
	memberAuthToken := sessionData.memberAuthToken
	sessionData.lock.Unlock()
	//----------------------------------------

	if fls == nil {
		return errors.New(string(FsNoFilesystem))
	}

	fpath, err := getFilePath(req.TextDocument.Uri, projectMode)
	if err != nil {
		return err
	}

	fullDocumentText := req.TextDocument.Text

	fsErr := fsutil.WriteFile(fls.unsavedDocumentsFS(), fpath, []byte(fullDocumentText), 0700)
	if fsErr != nil {
		logs.Println("failed to update state of document", fpath+":", fsErr)
	}

	registrationId := uuid.New()

	sessionData.lock.Lock()

	//create synchronization data if it does not exists
	_, hasSyncData := sessionData.unsavedDocumentSyncData[fpath]
	if !hasSyncData {
		sessionData.unsavedDocumentSyncData[fpath] = &unsavedDocumentSyncData{
			path: fpath,
		}
	}

	if _, ok := sessionData.didSaveCapabilityRegistrationIds[req.TextDocument.Uri]; !ok {
		sessionData.didSaveCapabilityRegistrationIds[req.TextDocument.Uri] = registrationId
		sessionData.lock.Unlock()

		session.SendRequest(jsonrpc.RequestMessage{
			Method: "client/registerCapability",
			Params: utils.Must(json.Marshal(defines.RegistrationParams{
				Registrations: []defines.Registration{
					{
						Id:     registrationId.String(),
						Method: "textDocument/didSave",
						RegisterOptions: defines.TextDocumentSaveRegistrationOptions{
							TextDocumentRegistrationOptions: defines.TextDocumentRegistrationOptions{
								DocumentSelector: req.TextDocument,
							},
							SaveOptions: defines.SaveOptions{
								IncludeText: &True,
							},
						},
					},
				},
			})),
		})

	} else {
		sessionData.lock.Unlock()
	}

	return notifyDiagnostics(diagnosticsParams{
		session:         session,
		docURI:          req.TextDocument.Uri,
		usingInoxFS:     projectMode,
		fls:             fls,
		memberAuthToken: memberAuthToken,
	})
}

func handleDidSaveDocument(ctx context.Context, req *defines.DidSaveTextDocumentParams) (err error) {
	session := jsonrpc.GetSession(ctx)

	//----------------------------------------
	sessionData := getLockedSessionData(session)
	projectMode := sessionData.projectMode
	fls := sessionData.filesystem
	memberAuthToken := sessionData.memberAuthToken

	fpath, err := getFilePath(req.TextDocument.Uri, projectMode)
	if err != nil {
		sessionData.lock.Unlock()
		return err
	}

	syncData, hasSyncData := sessionData.unsavedDocumentSyncData[fpath]
	sessionData.lock.Unlock()
	//----------------------------------------

	if fls == nil {
		return errors.New(string(FsNoFilesystem))
	}

	if hasSyncData {
		syncData.reactToDidChange(fls)
	}

	// The document's text should be included because we asked for it:
	// a client/registerCapability request was sent for the textDocument/didSave method.
	// After the document is saved we immediately unregister the capability. The only purpose is
	// to get the initial content for a newly created file as no textDocument/didChange request
	// is sent for the first modification.
	if req.Text != nil {
		fsErr := fsutil.WriteFile(fls.unsavedDocumentsFS(), fpath, []byte(*req.Text), 0700)
		if fsErr != nil {
			logs.Println("failed to update state of document", fpath+":", fsErr)
		}

		sessionData := getLockedSessionData(session)
		registrationId, ok := sessionData.didSaveCapabilityRegistrationIds[req.TextDocument.Uri]

		if !ok {
			sessionData.lock.Unlock()
		} else {
			delete(sessionData.didSaveCapabilityRegistrationIds, req.TextDocument.Uri)
			sessionData.lock.Unlock()

			session.SendRequest(jsonrpc.RequestMessage{
				Method: "client/unregisterCapability",
				ID:     uuid.New(),
				Params: utils.Must(json.Marshal(defines.UnregistrationParams{
					Unregistrations: []defines.Unregistration{
						{
							Id:     registrationId.String(),
							Method: "textDocument/didSave",
						},
					},
				})),
			})

			//on vscode unregistering the capability does not stop the client from sending didSave
			//notifications with the text included so we register the capability again but this time
			//we ask the client to not include the full text.

			session.SendRequest(jsonrpc.RequestMessage{
				Method: "client/registerCapability",
				Params: utils.Must(json.Marshal(defines.RegistrationParams{
					Registrations: []defines.Registration{
						{
							Id:     uuid.New().String(),
							Method: "textDocument/didSave",
							RegisterOptions: defines.TextDocumentSaveRegistrationOptions{
								TextDocumentRegistrationOptions: defines.TextDocumentRegistrationOptions{
									DocumentSelector: req.TextDocument,
								},
								SaveOptions: defines.SaveOptions{
									IncludeText: &False,
								},
							},
						},
					},
				})),
			})
		}
	}

	return notifyDiagnostics(diagnosticsParams{
		session:         session,
		docURI:          req.TextDocument.Uri,
		usingInoxFS:     projectMode,
		fls:             fls,
		memberAuthToken: memberAuthToken,
	})
}

func handleDidChangeDocument(ctx context.Context, req *defines.DidChangeTextDocumentParams) (err error) {
	session := jsonrpc.GetSession(ctx)

	//----------------------------------------
	sessionData := getLockedSessionData(session)
	projectMode := sessionData.projectMode
	fls := sessionData.filesystem
	memberAuthToken := sessionData.memberAuthToken

	fpath, err := getFilePath(req.TextDocument.Uri, projectMode)
	if err != nil {
		sessionData.lock.Unlock()
		return err
	}

	syncData, hasSyncData := sessionData.unsavedDocumentSyncData[fpath]
	sessionData.lock.Unlock()
	//----------------------------------------

	if fls == nil {
		return errors.New(string(FsNoFilesystem))
	}

	syncFull := sessionData.serverCapabilities.TextDocumentSync == defines.TextDocumentSyncKindFull
	var fullDocumentText string

	if hasSyncData {
		syncData.reactToDidChange(fls)
	}

	sessionData.preparedSourceFilesCache.acknowledgeSourceFileChange(fpath)

	if syncFull {
		fullDocumentText = req.ContentChanges[0].Text.(string)
	} else {
		beforeEditContent, err := fsutil.ReadFile(fls.unsavedDocumentsFS(), fpath)
		if err != nil {
			return jsonrpc.ResponseError{
				Code:    jsonrpc.InternalError.Code,
				Message: fmt.Sprintf("failed to read state of document %s: %s", fpath+":", err),
			}
		}

		beforeEditContentString := string(beforeEditContent)
		if sessionData.serverAPI != nil {
			sessionData.serverAPI.acknowledgeSourceFileChange(fpath, beforeEditContentString, req.ContentChanges)
		}

		var (
			lastReplacementStirng string
			lastRangeStart        int32
			lastRangeExlusiveEnd  int32
		)
		//TODO: minimize number and size of allocations.

		nextContent := []rune(beforeEditContentString)

		for _, change := range req.ContentChanges {
			startLine, startColumn := getLineColumn(change.Range.Start)
			endLine, endColumn := getLineColumn(change.Range.End)

			chunk, err := parse.ParseChunkSource(parse.InMemorySource{
				NameString: "script",
				CodeString: beforeEditContentString,
			})

			if err != nil && chunk == nil { //critical parsing error
				return jsonrpc.ResponseError{
					Code:    jsonrpc.InternalError.Code,
					Message: fmt.Sprintf("failed to update state of document %s: critical parsing error: %s", fpath+":", err),
				}
			}

			lastReplacementStirng = change.Text.(string)

			lastRangeStart = chunk.GetLineColumnPosition(startLine, startColumn)
			lastRangeExlusiveEnd = chunk.GetLineColumnPosition(endLine, endColumn)
			rangeLength := lastRangeExlusiveEnd - lastRangeStart

			afterRange := slices.Clone(nextContent[lastRangeStart+rangeLength:])
			nextContent = append(nextContent[:lastRangeStart], []rune(lastReplacementStirng)...)
			nextContent = append(nextContent, afterRange...)
		}

		fullDocumentText = string(nextContent)

		textEdit, ok := getAutoEditForChange(fullDocumentText, lastReplacementStirng, lastRangeStart, lastRangeExlusiveEnd)

		if ok {
			//the response can be safely ignored because if the edit is applied a textDocument/didSave request
			//will be sent by the client.
			go func() {
				defer utils.Recover()

				session.SendRequest(jsonrpc.RequestMessage{
					Method: "workspace/applyEdit",
					Params: utils.Must(json.Marshal(defines.ApplyWorkspaceEditParams{
						Edit: defines.WorkspaceEdit{
							Changes: &map[string][]defines.TextEdit{string(req.TextDocument.Uri): {textEdit}},
						},
					})),
				})

				time.Sleep(100 * time.Millisecond)

				session.SendRequest(jsonrpc.RequestMessage{
					Method: "cursor/setPosition",
					Params: utils.Must(json.Marshal(defines.Range{
						Start: textEdit.Range.Start,
						End:   textEdit.Range.Start,
					})),
				})
			}()
		}
	}

	fsErr := fsutil.WriteFile(fls.unsavedDocumentsFS(), fpath, []byte(fullDocumentText), 0700)

	if fsErr != nil {
		return jsonrpc.ResponseError{
			Code:    jsonrpc.InternalError.Code,
			Message: fmt.Sprintf("failed to update state of document %s: %s", fpath+":", fsErr),
		}
	}

	return notifyDiagnostics(diagnosticsParams{
		session:         session,
		docURI:          req.TextDocument.Uri,
		usingInoxFS:     projectMode,
		fls:             fls,
		memberAuthToken: memberAuthToken,
	})
}

func handleDidCloseDocument(ctx context.Context, req *defines.DidCloseTextDocumentParams) (err error) {
	session := jsonrpc.GetSession(ctx)

	//----------------------------------------
	sessionData := getLockedSessionData(session)
	projectMode := sessionData.projectMode
	fls := sessionData.filesystem

	if fls == nil {
		sessionData.lock.Unlock()
		return errors.New(string(FsNoFilesystem))
	}

	fpath, err := getFilePath(req.TextDocument.Uri, projectMode)
	if err != nil {
		sessionData.lock.Unlock()
		return err
	}

	delete(sessionData.unsavedDocumentSyncData, fpath)

	//NOTE: the file cache is not removed because other modules may still need it
	sessionData.lock.Unlock()
	//----------------------------------------

	docsFs := fls.unsavedDocumentsFS()
	if docsFs != fls {
		docsFs.Remove(fpath)
	}
	return nil
}

// IsLspSessionInitialized tells whether the 'initialize' method has been called by the client.
// Important: IsLspSessionInitialized locks/unlocks the session's data.
func IsLspSessionInitialized(session *jsonrpc.Session) bool {
	sessionData := getLockedSessionData(session)
	defer sessionData.lock.Unlock()
	return sessionData.clientCapabilities != (defines.ClientCapabilities{})
}
