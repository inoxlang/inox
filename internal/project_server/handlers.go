package project_server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"sync"

	fsutil "github.com/go-git/go-billy/v5/util"
	"github.com/google/uuid"

	core "github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/project_server/jsonrpc"
	"github.com/inoxlang/inox/internal/project_server/logs"
	"github.com/inoxlang/inox/internal/project_server/lsp"

	"github.com/inoxlang/inox/internal/project_server/lsp/defines"

	"github.com/inoxlang/inox/internal/globals/inox_ns"
	"github.com/inoxlang/inox/internal/utils"

	"github.com/inoxlang/inox/internal/globals/compl"
	parse "github.com/inoxlang/inox/internal/parse"

	_ "net/http/pprof"
	"net/url"
)

const (
	INOX_FS_SCHEME = "inox"
)

var (
	sessionToAdditionalData     = make(map[*jsonrpc.Session]*additionalSessionData)
	sessionToAdditionalDataLock sync.Mutex

	ErrFileURIExpected = errors.New("a file: URI was expected")
	ErrInoxURIExpected = errors.New("a inox: URI was expected")

	True  = true
	False = false
)

type additionalSessionData struct {
	lock sync.RWMutex

	didSaveCapabilityRegistrationIds map[defines.DocumentUri]uuid.UUID
	filesystem                       *Filesystem
	clientCapabilities               defines.ClientCapabilities
	serverCapabilities               defines.ServerCapabilities
	projectMode                      bool

	//debug adapter protocol
	debugSessions *DebugSessions
}

func (d *additionalSessionData) Scheme() string {
	if d.projectMode {
		return INOX_FS_SCHEME
	}
	return "file"
}

func getLockedSessionData(session *jsonrpc.Session) *additionalSessionData {
	sessionToAdditionalDataLock.Lock()
	sessionData := sessionToAdditionalData[session]
	if sessionData == nil {
		sessionData = &additionalSessionData{
			didSaveCapabilityRegistrationIds: make(map[defines.DocumentUri]uuid.UUID, 0),
		}
		sessionToAdditionalData[session] = sessionData
	}

	sessionToAdditionalDataLock.Unlock()
	sessionData.lock.Lock()
	return sessionData
}

func registerHandlers(server *lsp.Server, opts LSPServerOptions) {
	var (
		shuttingDownSessionsLock sync.Mutex
		shuttingDownSessions     = make(map[*jsonrpc.Session]struct{})
	)

	projectMode := opts.ProjectMode

	if projectMode {
		registerFilesystemMethodHandlers(server)
		registerProjectMethodHandlers(server, opts)
		registerDebugMethodHandlers(server, opts)
	}

	server.OnInitialize(func(ctx context.Context, req *defines.InitializeParams) (result *defines.InitializeResult, err *defines.InitializeError) {
		session := jsonrpc.GetSession(ctx)

		s := &defines.InitializeResult{}

		s.Capabilities.HoverProvider = true
		s.Capabilities.WorkspaceSymbolProvider = true
		s.Capabilities.DefinitionProvider = true
		s.Capabilities.CodeActionProvider = &defines.CodeActionOptions{
			CodeActionKinds: &[]defines.CodeActionKind{defines.CodeActionKindQuickFix},
		}

		if *req.Capabilities.TextDocument.Synchronization.DidSave && *req.Capabilities.TextDocument.Synchronization.DynamicRegistration {
			s.Capabilities.TextDocumentSync = defines.TextDocumentSyncKindIncremental
		} else {
			s.Capabilities.TextDocumentSync = defines.TextDocumentSyncKindFull
		}

		s.Capabilities.CompletionProvider = &defines.CompletionOptions{
			TriggerCharacters: &[]string{"."},
		}

		sessionData := getLockedSessionData(session)
		sessionData.clientCapabilities = req.Capabilities
		sessionData.serverCapabilities = s.Capabilities
		sessionData.projectMode = projectMode
		sessionData.lock.Unlock()

		return s, nil
	})

	server.OnShutdown(func(ctx context.Context, req *defines.NoParams) (err error) {
		session := jsonrpc.GetSession(ctx)

		shuttingDownSessionsLock.Lock()
		defer shuttingDownSessionsLock.Unlock()

		shuttingDownSessions[session] = struct{}{}
		return nil
	})

	server.OnExit(func(ctx context.Context, req *defines.NoParams) (err error) {
		session := jsonrpc.GetSession(ctx)

		shuttingDownSessionsLock.Lock()

		if _, ok := shuttingDownSessions[session]; ok {
			delete(shuttingDownSessions, session)
			shuttingDownSessionsLock.Unlock()

			sessionToAdditionalDataLock.Lock()
			delete(sessionToAdditionalData, session)
			sessionToAdditionalDataLock.Unlock()

			session.Close()
		} else {
			shuttingDownSessionsLock.Unlock()
			return errors.New("the client should make shutdown request before sending an exit notification")
		}

		return nil
	})

	server.OnHover(func(ctx context.Context, req *defines.HoverParams) (result *defines.Hover, err error) {
		fpath, err := getFilePath(req.TextDocument.Uri, projectMode)
		if err != nil {
			return nil, err
		}
		line, column := getLineColumn(req.Position)
		session := jsonrpc.GetSession(ctx)
		sessionCtx := session.Context()

		fls, ok := getLspFilesystem(session)
		if !ok {
			return nil, errors.New(FsNoFilesystem)
		}

		handlingCtx := sessionCtx.BoundChildWithOptions(core.BoundChildContextOptions{
			Filesystem: fls,
		})

		return getHoverContent(fpath, line, column, handlingCtx)
	})

	server.OnCompletion(func(ctx context.Context, req *defines.CompletionParams) (result *[]defines.CompletionItem, err error) {
		fpath, err := getFilePath(req.TextDocument.Uri, projectMode)
		if err != nil {
			return nil, err
		}

		line, column := getLineColumn(req.Position)
		session := jsonrpc.GetSession(ctx)

		completions := getCompletions(fpath, line, column, session)
		completionIndex := 0

		lspCompletions := utils.MapSlice(completions, func(completion compl.Completion) defines.CompletionItem {
			defer func() {
				completionIndex++
			}()

			var labelDetails *defines.CompletionItemLabelDetails
			if completion.Detail != "" {
				detail := "  " + completion.Detail
				labelDetails = &defines.CompletionItemLabelDetails{
					Detail: &detail,
				}
			}

			return defines.CompletionItem{
				Label: completion.Value,
				Kind:  &completion.Kind,
				TextEdit: defines.TextEdit{
					Range: rangeToLspRange(completion.ReplacedRange),
				},
				SortText: func() *string {
					index := completionIndex
					if index > 99 {
						index = 99
					}
					s := string(rune(index/10) + 'a')
					s += string(rune(index%10) + 'a')
					return &s
				}(),
				LabelDetails: labelDetails,
			}
		})
		return &lspCompletions, nil
	})

	server.OnCodeActionWithSliceCodeAction(func(ctx context.Context, req *defines.CodeActionParams) (result *[]defines.CodeAction, err error) {
		fpath, err := getFilePath(req.TextDocument.Uri, projectMode)
		if err != nil {
			return nil, err
		}

		session := jsonrpc.GetSession(ctx)
		fls, ok := getLspFilesystem(session)
		if !ok {
			return nil, nil
		}

		actions, err := getCodeActions(session, req.Context.Diagnostics, req.Range, req.TextDocument, fpath, fls)

		if err != nil {
			return nil, jsonrpc.ResponseError{
				Code:    jsonrpc.InternalError.Code,
				Message: fmt.Sprintf("failed to get code actions: %s", err),
			}
		}
		return actions, nil
	})

	server.OnDidOpenTextDocument(func(ctx context.Context, req *defines.DidOpenTextDocumentParams) (err error) {
		fpath, err := getFilePath(req.TextDocument.Uri, projectMode)
		if err != nil {
			return err
		}

		fullDocumentText := req.TextDocument.Text
		session := jsonrpc.GetSession(ctx)
		fls, ok := getLspFilesystem(session)
		if !ok {
			return errors.New(FsNoFilesystem)
		}

		fsErr := fsutil.WriteFile(fls.docsFS(), fpath, []byte(fullDocumentText), 0700)
		if fsErr != nil {
			logs.Println("failed to update state of document", fpath+":", fsErr)
		}

		registrationId := uuid.New()
		sessionData := getLockedSessionData(session)

		if _, ok := sessionData.didSaveCapabilityRegistrationIds[req.TextDocument.Uri]; !ok {
			sessionData.didSaveCapabilityRegistrationIds[req.TextDocument.Uri] = registrationId
			sessionData.lock.Unlock()

			session.SendRequest(jsonrpc.RequestMessage{
				BaseMessage: jsonrpc.BaseMessage{
					Jsonrpc: JSONRPC_VERSION,
				},
				Method: "client/registerCapability",
				ID:     uuid.New(),
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

		return notifyDiagnostics(session, req.TextDocument.Uri, projectMode, fls)
	})

	server.OnDidSaveTextDocument(func(ctx context.Context, req *defines.DidSaveTextDocumentParams) (err error) {
		fpath, err := getFilePath(req.TextDocument.Uri, projectMode)
		if err != nil {
			return err
		}
		session := jsonrpc.GetSession(ctx)
		fls, ok := getLspFilesystem(session)
		if !ok {
			return errors.New(FsNoFilesystem)
		}

		// The document's text should be included because we asked for it:
		// a client/registerCapability request was sent for the textDocument/didSave method.
		// After the document is saved we immediately unregister the capability. The only purpose is
		// to get the initial content for a newly created file as no textDocument/didChange request
		// is sent for the first modification.
		if req.Text != nil {
			fsErr := fsutil.WriteFile(fls.docsFS(), fpath, []byte(*req.Text), 0700)
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
					BaseMessage: jsonrpc.BaseMessage{
						Jsonrpc: JSONRPC_VERSION,
					},
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
					BaseMessage: jsonrpc.BaseMessage{
						Jsonrpc: JSONRPC_VERSION,
					},
					Method: "client/registerCapability",
					ID:     uuid.New(),
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

		return notifyDiagnostics(session, req.TextDocument.Uri, projectMode, fls)
	})

	server.OnDidChangeTextDocument(func(ctx context.Context, req *defines.DidChangeTextDocumentParams) (err error) {
		fpath, err := getFilePath(req.TextDocument.Uri, projectMode)
		if err != nil {
			return err
		}

		session := jsonrpc.GetSession(ctx)
		fls, ok := getLspFilesystem(session)
		if !ok {
			return errors.New(FsNoFilesystem)
		}

		var fullDocumentText string

		sessionData := getLockedSessionData(session)
		syncFull := sessionData.serverCapabilities.TextDocumentSync == defines.TextDocumentSyncKindFull
		sessionData.lock.Unlock()

		if syncFull {
			fullDocumentText = req.ContentChanges[0].Text.(string)
		} else {
			currentContent, err := fsutil.ReadFile(fls.docsFS(), fpath)
			if err != nil {
				return jsonrpc.ResponseError{
					Code:    jsonrpc.InternalError.Code,
					Message: fmt.Sprintf("failed to read state of document %s: %s", fpath+":", err),
				}
			}

			var (
				lastReplacementStirng string
				lastRangeStart        int32
				lastRangeExlusiveEnd  int32
			)

			for _, change := range req.ContentChanges {
				startLine, startColumn := getLineColumn(change.Range.Start)
				endLine, endColumn := getLineColumn(change.Range.End)

				chunk, err := parse.ParseChunkSource(parse.InMemorySource{
					NameString: "script",
					CodeString: string(currentContent),
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

				afterRange := utils.CopySlice(currentContent[lastRangeStart+rangeLength:])
				currentContent = append(currentContent[:lastRangeStart], lastReplacementStirng...)
				currentContent = append(currentContent, afterRange...)
			}

			fullDocumentText = string(currentContent)

			textEdit, ok := getAutoEditForChange(fullDocumentText, lastReplacementStirng, lastRangeStart, lastRangeExlusiveEnd)

			if ok {
				//the response can be sefaly ignored because if the edit is applied a textDocument/didSave request
				//will be sent by the client.
				go session.SendRequest(jsonrpc.RequestMessage{
					BaseMessage: jsonrpc.BaseMessage{
						Jsonrpc: JSONRPC_VERSION,
					},
					Method: "workspace/applyEdit",
					ID:     uuid.New(),
					Params: utils.Must(json.Marshal(defines.ApplyWorkspaceEditParams{
						Edit: defines.WorkspaceEdit{
							Changes: &map[string][]defines.TextEdit{
								string(req.TextDocument.Uri): {textEdit},
							},
						},
					})),
				})
			}
		}

		fsErr := fsutil.WriteFile(fls.docsFS(), fpath, []byte(fullDocumentText), 0700)
		if fsErr != nil {
			return jsonrpc.ResponseError{
				Code:    jsonrpc.InternalError.Code,
				Message: fmt.Sprintf("failed to update state of document %s: %s", fpath+":", fsErr),
			}
		}

		return notifyDiagnostics(session, req.TextDocument.Uri, projectMode, fls)
	})

	server.OnDefinition(func(ctx context.Context, req *defines.DefinitionParams) (result *[]defines.LocationLink, err error) {
		fpath, err := getFilePath(req.TextDocument.Uri, projectMode)
		if err != nil {
			return nil, err
		}
		line, column := getLineColumn(req.Position)
		session := jsonrpc.GetSession(ctx)
		sessionCtx := session.Context()
		sessionData := getLockedSessionData(session)
		sessionData.lock.Unlock()

		fls, ok := getLspFilesystem(session)
		if !ok {
			return nil, errors.New(FsNoFilesystem)
		}

		handlingCtx := sessionCtx.BoundChildWithOptions(core.BoundChildContextOptions{
			Filesystem: fls,
		})

		state, mod, _, err := inox_ns.PrepareLocalScript(inox_ns.ScriptPreparationArgs{
			Fpath:                     fpath,
			ParsingCompilationContext: handlingCtx,
			ParentContext:             nil,
			Out:                       io.Discard,
			DevMode:                   true,
			AllowMissingEnvVars:       true,
			ScriptContextFileSystem:   fls,
			PreinitFilesystem:         fls,
		})

		if state == nil || state.SymbolicData == nil {
			logs.Println("failed to prepare script", err)
			return nil, nil
		}

		//TODO: support definition when included chunk is being edited
		chunk := mod.MainChunk

		span := chunk.GetLineColumnSingeCharSpan(line, column)
		foundNode, ancestors, ok := chunk.GetNodeAndChainAtSpan(span)

		if !ok || foundNode == nil {
			logs.Println("no data: node not found")
			return nil, nil
		}

		var position parse.SourcePositionRange

		ok = false

		switch n := foundNode.(type) {
		case *parse.Variable, *parse.GlobalVariable, *parse.IdentifierLiteral:
			position, ok = state.SymbolicData.GetVariableDefinitionPosition(foundNode, ancestors)

		case *parse.PatternIdentifierLiteral, *parse.PatternNamespaceIdentifierLiteral:
			position, ok = state.SymbolicData.GetNamedPatternOrPatternNamespacePositionDefinition(foundNode, ancestors)
		case *parse.RelativePathLiteral:
			parent := ancestors[len(ancestors)-1]
			switch parent.(type) {
			case *parse.InclusionImportStatement:
				file, isFile := chunk.Source.(parse.SourceFile)
				if !isFile || file.IsResourceURL || file.ResourceDir == "" {
					break
				}

				path := filepath.Join(file.ResourceDir, n.Value)
				position = parse.SourcePositionRange{
					SourceName:  path,
					StartLine:   1,
					StartColumn: 1,
					EndLine:     1,
					EndColumn:   2,
					Span:        parse.NodeSpan{Start: 0, End: 1},
				}
				ok = true
			}
		}

		if !ok {
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
	})

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
	return u.Path, nil
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
	return u.Path, nil
}

func rangeToLspRange(r parse.SourcePositionRange) defines.Range {
	return defines.Range{
		Start: defines.Position{
			Line:      uint(r.StartLine) - 1,
			Character: uint(r.StartColumn - 1),
		},
		//exclusive end
		End: defines.Position{
			Line:      uint(r.EndLine - 1),
			Character: uint(r.EndColumn - 1),
		},
	}
}

func firstCharsLspRange(count int32) defines.Range {
	return rangeToLspRange(parse.SourcePositionRange{
		StartLine:   1,
		StartColumn: 1,
		EndLine:     1,
		EndColumn:   1,
		Span:        parse.NodeSpan{Start: 0, End: count},
	})
}

func getLineColumn(pos defines.Position) (int32, int32) {
	line := int32(pos.Line + 1)
	column := int32(pos.Character + 1)
	return line, column
}
