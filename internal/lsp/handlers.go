package internal

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"sync"

	fsutil "github.com/go-git/go-billy/v5/util"
	"github.com/google/uuid"

	core "github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/lsp/jsonrpc"
	"github.com/inoxlang/inox/internal/lsp/logs"
	"github.com/inoxlang/inox/internal/lsp/lsp"

	"github.com/inoxlang/inox/internal/lsp/lsp/defines"

	"github.com/inoxlang/inox/internal/globals/inox_ns"
	"github.com/inoxlang/inox/internal/utils"

	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/globals/compl"
	help_ns "github.com/inoxlang/inox/internal/globals/help_ns"
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
	lock                             sync.RWMutex
	didSaveCapabilityRegistrationIds map[defines.DocumentUri]uuid.UUID
	filesystem                       *Filesystem

	//debug adapter protocol
	debugSessions *DebugSessions
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
		logs.Println("initialized")
		s := &defines.InitializeResult{}

		s.Capabilities.HoverProvider = true
		s.Capabilities.WorkspaceSymbolProvider = true
		s.Capabilities.DefinitionProvider = true

		if *req.Capabilities.TextDocument.Synchronization.DidSave && *req.Capabilities.TextDocument.Synchronization.DynamicRegistration {
			s.Capabilities.TextDocumentSync = defines.TextDocumentSyncKindIncremental
		} else {
			s.Capabilities.TextDocumentSync = defines.TextDocumentSyncKindFull
		}

		s.Capabilities.CompletionProvider = &defines.CompletionOptions{
			TriggerCharacters: &[]string{"."},
		}
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

		state, mod, _, _ := inox_ns.PrepareLocalScript(inox_ns.ScriptPreparationArgs{
			Fpath:                     fpath,
			ParsingCompilationContext: handlingCtx,
			ParentContext:             nil,
			Out:                       io.Discard,
			DevMode:                   true,
			AllowMissingEnvVars:       true,
			PreinitFilesystem:         fls,
			ScriptContextFileSystem:   fls,
		})

		if state == nil || state.SymbolicData == nil {
			logs.Println("no data")
			return &defines.Hover{}, nil
		}

		span := mod.MainChunk.GetLineColumnSingeCharSpan(line, column)
		foundNode, ok := mod.MainChunk.GetNodeAtSpan(span)

		if !ok || foundNode == nil {
			logs.Println("no data")
			return &defines.Hover{}, nil
		}

		mostSpecificVal, ok := state.SymbolicData.GetMostSpecificNodeValue(foundNode)
		var lessSpecificVal symbolic.SymbolicValue
		if !ok {
			logs.Println("no data")
			return &defines.Hover{}, nil
		}

		buff := &bytes.Buffer{}
		w := bufio.NewWriterSize(buff, 1000)
		var stringified string
		{
			utils.PanicIfErr(symbolic.PrettyPrint(mostSpecificVal, w, HOVER_PRETTY_PRINT_CONFIG, 0, 0))
			var ok bool
			lessSpecificVal, ok = state.SymbolicData.GetLessSpecificNodeValue(foundNode)
			if ok {
				w.Write(utils.StringAsBytes("\n\n# less specific\n"))
				utils.PanicIfErr(symbolic.PrettyPrint(lessSpecificVal, w, HOVER_PRETTY_PRINT_CONFIG, 0, 0))
			}

			w.Flush()
			stringified = strings.ReplaceAll(buff.String(), "\n\r", "\n")
			logs.Println(stringified)
		}

		//help
		var helpMessage string
		{
			val := mostSpecificVal
			for {
				switch val := val.(type) {
				case *symbolic.GoFunction:
					text, ok := help_ns.HelpForSymbolicGoFunc(val, help_ns.HelpMessageConfig{Format: help_ns.MarkdownFormat})
					if ok {
						helpMessage = "\n-----\n" + strings.ReplaceAll(text, "\n\r", "\n")
					}
				}
				if helpMessage == "" && val == mostSpecificVal && lessSpecificVal != nil {
					val = lessSpecificVal
					continue
				}
				break
			}

		}

		return &defines.Hover{
			Contents: defines.MarkupContent{
				Kind:  defines.MarkupKindMarkdown,
				Value: "```inox\n" + stringified + "\n```" + helpMessage,
			},
		}, nil
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

		//full document text
		if len(req.ContentChanges) == 1 && req.ContentChanges[0].Range == (defines.Range{}) {
			fullDocumentText = req.ContentChanges[0].Text.(string)
		} else {
			currentContent, err := fsutil.ReadFile(fls.docsFS(), fpath)
			if err != nil {
				return jsonrpc.ResponseError{
					Code:    jsonrpc.InternalError.Code,
					Message: fmt.Sprintf("failed to read state of document %s: %s", fpath+":", err),
				}
			}

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

				start := chunk.GetLineColumnPosition(startLine, startColumn)
				exclusiveEnd := chunk.GetLineColumnPosition(endLine, endColumn)
				rangeLength := exclusiveEnd - start

				replacement := change.Text.(string)

				afterRange := utils.CopySlice(currentContent[start+rangeLength:])
				currentContent = append(currentContent[:start], replacement...)
				currentContent = append(currentContent, afterRange...)
			}

			fullDocumentText = string(currentContent)
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
				TargetUri:            defines.DocumentUri("file://" + position.SourceName),
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

func getCompletions(fpath string, line, column int32, session *jsonrpc.Session) []compl.Completion {
	fls, ok := getLspFilesystem(session)
	if !ok {
		return nil
	}

	handlingCtx := session.Context().BoundChildWithOptions(core.BoundChildContextOptions{
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

	if mod == nil { //unrecoverable parsing error
		logs.Println("unrecoverable parsing error", err.Error())
		session.Notify(NewShowMessage(defines.MessageTypeError, err.Error()))
		return nil
	}

	if state == nil {
		logs.Println("error", err.Error())
		session.Notify(NewShowMessage(defines.MessageTypeError, err.Error()))
		return nil
	}

	chunk := mod.MainChunk
	pos := chunk.GetLineColumnPosition(line, column)

	return compl.FindCompletions(compl.CompletionSearchArgs{
		State:       core.NewTreeWalkStateWithGlobal(state),
		Chunk:       chunk,
		CursorIndex: int(pos),
		Mode:        compl.LspCompletions,
	})
}

func rangeToLspRange(r parse.SourcePositionRange) defines.Range {
	return defines.Range{
		Start: defines.Position{
			Line:      uint(r.StartLine) - 1,
			Character: uint(r.StartColumn - 1),
		},
		End: defines.Position{
			Line:      uint(r.StartLine) - 1,
			Character: uint(r.StartColumn - 1 + r.Span.End - r.Span.Start),
		},
	}
}

func firstCharsLspRange(count int32) defines.Range {
	return rangeToLspRange(parse.SourcePositionRange{
		StartLine:   1,
		StartColumn: 1,
		Span:        parse.NodeSpan{Start: 0, End: count},
	})
}

func getLineColumn(pos defines.Position) (int32, int32) {
	line := int32(pos.Line + 1)
	column := int32(pos.Character + 1)
	return line, column
}
