package internal

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"os"
	"strings"

	fsutil "github.com/go-git/go-billy/v5/util"
	core "github.com/inoxlang/inox/internal/core"
	symbolic "github.com/inoxlang/inox/internal/core/symbolic"
	pprint "github.com/inoxlang/inox/internal/pretty_print"

	parse "github.com/inoxlang/inox/internal/parse"

	"github.com/inoxlang/inox/internal/lsp/jsonrpc"
	"github.com/inoxlang/inox/internal/lsp/logs"
	"github.com/inoxlang/inox/internal/lsp/lsp"

	"github.com/inoxlang/inox/internal/lsp/lsp/defines"

	"github.com/inoxlang/inox/internal/utils"

	globals "github.com/inoxlang/inox/internal/globals"
	compl "github.com/inoxlang/inox/internal/globals/completion"

	_ "net/http/pprof"
	"net/url"
)

var HOVER_PRETTY_PRINT_CONFIG = &pprint.PrettyPrintConfig{
	MaxDepth: 7,
	Indent:   []byte{' ', ' '},
	Colorize: false,
	Compact:  false,
}

func StartLSPServer() {

	f, err := os.OpenFile("/tmp/.inox-lsp.debug.txt", os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o600)
	if err != nil {
		log.Panicln(err)
	}

	logger := log.New(f, "", 0)
	logs.Init(logger)

	defer func() {
		e := recover()

		if e != nil {
			logs.Println(e)
		}

		f.Close()
	}()

	server := lsp.NewServer(&lsp.Options{
		CompletionProvider: &defines.CompletionOptions{
			TriggerCharacters: &[]string{"."},
		},
		TextDocumentSync: defines.TextDocumentSyncKindFull,
	})

	filesystem := NewFilesystem()

	compilationCtx := core.NewContext(core.ContextConfig{
		Permissions: []core.Permission{
			//TODO: change path pattern
			core.FilesystemPermission{Kind_: core.ReadPerm, Entity: core.PathPattern("/...")},
		},
		Filesystem: filesystem,
	})
	core.NewGlobalState(compilationCtx)

	server.OnHover(func(ctx context.Context, req *defines.HoverParams) (result *defines.Hover, err error) {
		logs.Println(req)

		fpath := getFilePath(req.TextDocument.Uri)
		line := int32(req.Position.Line + 1)
		column := int32(req.Position.Character + 1)

		state, mod, _ := globals.PrepareLocalScript(globals.ScriptPreparationArgs{
			Fpath:                     fpath,
			ParsingCompilationContext: compilationCtx,
			ParentContext:             nil,
			Out:                       os.Stdout,
			IgnoreNonCriticalIssues:   true,
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

		primaryVal, ok := state.SymbolicData.GetMostSpecificNodeValue(foundNode)
		if !ok {
			logs.Println("no data")
			return &defines.Hover{}, nil
		}

		buff := &bytes.Buffer{}
		w := bufio.NewWriterSize(buff, 1000)
		var stringified string
		{
			utils.PanicIfErr(symbolic.PrettyPrint(primaryVal, w, HOVER_PRETTY_PRINT_CONFIG, 0, 0))

			secondaryVal, ok := state.SymbolicData.GetLessSpecificNodeValue(foundNode)
			if ok {
				w.Write(utils.StringAsBytes("\n\n# less specific\n"))
				utils.PanicIfErr(symbolic.PrettyPrint(secondaryVal, w, HOVER_PRETTY_PRINT_CONFIG, 0, 0))
			}

			w.Flush()
			stringified = strings.ReplaceAll(buff.String(), "\n\r", "\n")
			logs.Println(stringified)
		}

		return &defines.Hover{
			Contents: defines.MarkupContent{
				Kind:  defines.MarkupKindMarkdown,
				Value: "```inox\n" + stringified + "\n```",
			},
		}, nil
	})

	server.OnCompletion(func(ctx context.Context, req *defines.CompletionParams) (result *[]defines.CompletionItem, err error) {
		fpath := getFilePath(req.TextDocument.Uri)
		line := int32(req.Position.Line + 1)
		column := int32(req.Position.Character + 1)

		completions := getCompletions(fpath, compilationCtx, line, column)
		completionIndex := 0

		lspCompletions := utils.MapSlice(completions, func(completion compl.Completion) defines.CompletionItem {
			defer func() {
				completionIndex++
			}()
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
			}
		})
		return &lspCompletions, nil
	})

	server.OnDidOpenTextDocument(func(ctx context.Context, req *defines.DidOpenTextDocumentParams) (err error) {
		fpath := getFilePath(req.TextDocument.Uri)
		fullDocumentText := req.TextDocument.Text

		fsErr := fsutil.WriteFile(filesystem.documents, fpath, []byte(fullDocumentText), 0700)
		if fsErr != nil {
			logs.Println("failed to update state of document", fpath+":", fsErr)
		}

		session := jsonrpc.GetSession(ctx)
		return notifyDiagnostics(session, req.TextDocument.Uri, compilationCtx)
	})

	server.OnDidChangeTextDocument(func(ctx context.Context, req *defines.DidChangeTextDocumentParams) (err error) {
		fpath := getFilePath(req.TextDocument.Uri)
		if len(req.ContentChanges) > 1 {
			return errors.New("single change supported")
		}
		fullDocumentText := req.ContentChanges[0].Text.(string)
		fsErr := fsutil.WriteFile(filesystem.documents, fpath, []byte(fullDocumentText), 0700)
		if fsErr != nil {
			logs.Println("failed to update state of document", fpath+":", fsErr)
		}

		session := jsonrpc.GetSession(ctx)
		return notifyDiagnostics(session, req.TextDocument.Uri, compilationCtx)
	})

	server.OnInitialize(func(ctx context.Context, req *defines.InitializeParams) (result *defines.InitializeResult, err *defines.InitializeError) {
		logs.Println("initialized")
		s := &defines.InitializeResult{}

		s.Capabilities.HoverProvider = true

		s.Capabilities.WorkspaceSymbolProvider = true

		// makes the client send the whole document during synchronization
		s.Capabilities.TextDocumentSync = defines.TextDocumentSyncKindFull

		s.Capabilities.CompletionProvider = &defines.CompletionOptions{}
		return s, nil
	})

	server.Run()
}

func getFilePath(uri defines.DocumentUri) string {
	return utils.Must(url.Parse(string(uri))).Path
}

func notifyDiagnostics(session *jsonrpc.Session, docURI defines.DocumentUri, compilationCtx *core.Context) error {
	fpath := getFilePath(docURI)

	errSeverity := defines.DiagnosticSeverityError
	state, mod, err := globals.PrepareLocalScript(globals.ScriptPreparationArgs{
		Fpath:                     fpath,
		ParsingCompilationContext: compilationCtx,
		ParentContext:             nil,
		Out:                       io.Discard,
		IgnoreNonCriticalIssues:   true,
	})

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

		if state != nil && state.StaticCheckData != nil {
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
			symbolicCheckDiagnostics := utils.MapSlice(state.SymbolicData.Errors(), func(err symbolic.SymbolicEvaluationError) defines.Diagnostic {
				i++

				return defines.Diagnostic{
					Message:  err.Message,
					Severity: &errSeverity,
					Range:    rangeToLspRange(err.Location[0]),
				}
			})

			diagnostics = append(diagnostics, symbolicCheckDiagnostics...)
		}
	}

send_diagnostics:
	session.Notify(jsonrpc.NotificationMessage{
		BaseMessage: jsonrpc.BaseMessage{
			Jsonrpc: "2.0",
		},
		Method: "textDocument/publishDiagnostics",
		Params: utils.Must(json.Marshal(defines.PublishDiagnosticsParams{
			Uri:         docURI,
			Diagnostics: diagnostics,
		})),
	})

	return nil
}

func getCompletions(fpath string, compilationCtx *core.Context, line, column int32) []compl.Completion {
	state, mod, _ := globals.PrepareLocalScript(globals.ScriptPreparationArgs{
		Fpath:                     fpath,
		ParsingCompilationContext: compilationCtx,
		ParentContext:             nil,
		Out:                       os.Stdout,
		IgnoreNonCriticalIssues:   true,
	})

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
