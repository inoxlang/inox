package internal

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"

	fsutil "github.com/go-git/go-billy/v5/util"

	core "github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/lsp/jsonrpc"
	"github.com/inoxlang/inox/internal/lsp/logs"
	"github.com/inoxlang/inox/internal/lsp/lsp"

	"github.com/inoxlang/inox/internal/lsp/lsp/defines"

	"github.com/inoxlang/inox/internal/utils"

	symbolic "github.com/inoxlang/inox/internal/core/symbolic"
	globals "github.com/inoxlang/inox/internal/globals"
	compl "github.com/inoxlang/inox/internal/globals/completion"
	help "github.com/inoxlang/inox/internal/globals/help"
	parse "github.com/inoxlang/inox/internal/parse"

	_ "net/http/pprof"
	"net/url"
)

func registerHandlers(server *lsp.Server, filesystem *Filesystem, compilationCtx *core.Context) {

	server.OnInitialize(func(ctx context.Context, req *defines.InitializeParams) (result *defines.InitializeResult, err *defines.InitializeError) {
		logs.Println("initialized")
		s := &defines.InitializeResult{}

		s.Capabilities.HoverProvider = true
		s.Capabilities.WorkspaceSymbolProvider = true
		s.Capabilities.DefinitionProvider = true

		// makes the client send the whole document during synchronization
		s.Capabilities.TextDocumentSync = defines.TextDocumentSyncKindFull

		s.Capabilities.CompletionProvider = &defines.CompletionOptions{}
		return s, nil
	})

	server.OnHover(func(ctx context.Context, req *defines.HoverParams) (result *defines.Hover, err error) {
		fpath := getFilePath(req.TextDocument.Uri)
		line, column := getLineColumn(req.Position)

		state, mod, _ := globals.PrepareLocalScript(globals.ScriptPreparationArgs{
			Fpath:                     fpath,
			ParsingCompilationContext: compilationCtx,
			ParentContext:             nil,
			Out:                       os.Stdout,
			IgnoreNonCriticalIssues:   true,
			AllowMissingEnvVars:       true,
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
					text, ok := help.HelpForSymbolicGoFunc(val, help.HelpMessageConfig{Format: help.MarkdownFormat})
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
		fpath := getFilePath(req.TextDocument.Uri)
		line, column := getLineColumn(req.Position)
		session := jsonrpc.GetSession(ctx)

		completions := getCompletions(fpath, compilationCtx, line, column, session)
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

	server.OnDefinition(func(ctx context.Context, req *defines.DefinitionParams) (result *[]defines.LocationLink, err error) {
		fpath := getFilePath(req.TextDocument.Uri)
		line, column := getLineColumn(req.Position)

		state, mod, err := globals.PrepareLocalScript(globals.ScriptPreparationArgs{
			Fpath:                     fpath,
			ParsingCompilationContext: compilationCtx,
			ParentContext:             nil,
			Out:                       os.Stdout,
			IgnoreNonCriticalIssues:   true,
			AllowMissingEnvVars:       true,
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

func getFilePath(uri defines.DocumentUri) string {
	return utils.Must(url.Parse(string(uri))).Path
}

func getCompletions(fpath string, compilationCtx *core.Context, line, column int32, session *jsonrpc.Session) []compl.Completion {
	state, mod, err := globals.PrepareLocalScript(globals.ScriptPreparationArgs{
		Fpath:                     fpath,
		ParsingCompilationContext: compilationCtx,
		ParentContext:             nil,
		Out:                       os.Stdout,
		IgnoreNonCriticalIssues:   true,
		AllowMissingEnvVars:       true,
	})

	if mod == nil { //unrecoverable error
		logs.Println("unrecoverable error", err.Error())
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

func getLineColumn(pos defines.Position) (int32, int32) {
	line := int32(pos.Line + 1)
	column := int32(pos.Character + 1)
	return line, column
}
