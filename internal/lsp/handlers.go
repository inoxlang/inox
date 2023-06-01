package internal

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	fsutil "github.com/go-git/go-billy/v5/util"

	"github.com/inoxlang/inox/internal/afs"
	core "github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/lsp/jsonrpc"
	"github.com/inoxlang/inox/internal/lsp/logs"
	"github.com/inoxlang/inox/internal/lsp/lsp"

	"github.com/inoxlang/inox/internal/lsp/lsp/defines"

	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/inoxlang/inox/internal/globals/inox_ns"
	"github.com/inoxlang/inox/internal/utils"

	symbolic "github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/globals/compl"
	help_ns "github.com/inoxlang/inox/internal/globals/help_ns"
	parse "github.com/inoxlang/inox/internal/parse"

	_ "net/http/pprof"
	"net/url"
)

func registerHandlers(server *lsp.Server, registerFilesystemHandlers bool) {

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
		session := jsonrpc.GetSession(ctx)

		state, mod, _, _ := inox_ns.PrepareLocalScript(inox_ns.ScriptPreparationArgs{
			Fpath:                     fpath,
			ParsingCompilationContext: session.Context(),
			ParentContext:             nil,
			Out:                       os.Stdout,
			IgnoreNonCriticalIssues:   true,
			AllowMissingEnvVars:       true,
			ScriptContextFileSystem:   session.Context().GetFileSystem(),
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
		fpath := getFilePath(req.TextDocument.Uri)
		line, column := getLineColumn(req.Position)
		session := jsonrpc.GetSession(ctx)

		completions := getCompletions(fpath, session.Context(), line, column, session, session.Context().GetFileSystem())
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
		session := jsonrpc.GetSession(ctx)
		fls := session.Context().GetFileSystem().(*Filesystem)

		fsErr := fsutil.WriteFile(fls.docsFS(), fpath, []byte(fullDocumentText), 0700)
		if fsErr != nil {
			logs.Println("failed to update state of document", fpath+":", fsErr)
		}

		return notifyDiagnostics(session, req.TextDocument.Uri, session.Context(), fls)
	})

	server.OnDidChangeTextDocument(func(ctx context.Context, req *defines.DidChangeTextDocumentParams) (err error) {
		fpath := getFilePath(req.TextDocument.Uri)
		if len(req.ContentChanges) > 1 {
			return errors.New("single change supported")
		}
		session := jsonrpc.GetSession(ctx)
		fls := session.Context().GetFileSystem().(*Filesystem)

		fullDocumentText := req.ContentChanges[0].Text.(string)
		fsErr := fsutil.WriteFile(fls.docsFS(), fpath, []byte(fullDocumentText), 0700)
		if fsErr != nil {
			logs.Println("failed to update state of document", fpath+":", fsErr)
		}

		return notifyDiagnostics(session, req.TextDocument.Uri, session.Context(), fls)
	})

	server.OnDefinition(func(ctx context.Context, req *defines.DefinitionParams) (result *[]defines.LocationLink, err error) {
		fpath := getFilePath(req.TextDocument.Uri)
		line, column := getLineColumn(req.Position)
		session := jsonrpc.GetSession(ctx)

		state, mod, _, err := inox_ns.PrepareLocalScript(inox_ns.ScriptPreparationArgs{
			Fpath:                     fpath,
			ParsingCompilationContext: session.Context(),
			ParentContext:             nil,
			Out:                       os.Stdout,
			IgnoreNonCriticalIssues:   true,
			AllowMissingEnvVars:       true,
			ScriptContextFileSystem:   session.Context().GetFileSystem(),
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

	if registerFilesystemHandlers {
		server.OnCustom(jsonrpc.MethodInfo{
			Name: "fs/fileStat",
			NewRequest: func() interface{} {
				return &FsFileStatParams{}
			},
			Handler: func(ctx context.Context, req interface{}) (interface{}, error) {
				session := jsonrpc.GetSession(ctx)
				fls := session.Context().GetFileSystem()
				params := req.(*FsFileStatParams)

				u, err := url.Parse(string(params.FileURI))
				if err != nil {
					logs.Println(err)
					return nil, nil
				}

				stat, err := fls.Stat(u.Path)
				if err != nil {
					return nil, fmt.Errorf("failed to get stat for file %s", u.Path)
				}

				ctime, mtime, err := fs_ns.GetCreationAndModifTime(stat)
				if err != nil {
					return nil, fmt.Errorf("failed to get ctime/mtime for file %s", u.Path)
				}

				return &FsFileStat{
					CTime:    ctime.UnixMilli(),
					MTime:    mtime.UnixMilli(),
					Size:     stat.Size(),
					FileType: FsDir,
				}, nil
			},
		})

		server.OnCustom(jsonrpc.MethodInfo{
			Name: "fs/readDir",
			NewRequest: func() interface{} {
				return &FsReadirParams{}
			},
			Handler: func(ctx context.Context, req interface{}) (interface{}, error) {
				session := jsonrpc.GetSession(ctx)
				fls := session.Context().GetFileSystem()
				params := req.(*FsReadirParams)

				u, err := url.Parse(string(params.DirURI))
				if err != nil {
					logs.Println(err)
					return nil, nil
				}

				entries, err := fls.ReadDir(u.Path)
				if err != nil {
					return nil, fmt.Errorf("failed to read dir %s", u.Path)
				}

				var fsDirEntries FsDirEntries
				for _, e := range entries {
					fsDirEntries = append(fsDirEntries, FsDirEntry{
						Name:     e.Name(),
						FileType: FileTypeFromInfo(e),
					})
				}

				return fsDirEntries, nil
			},
		})
	}

}

func getFilePath(uri defines.DocumentUri) string {
	return utils.Must(url.Parse(string(uri))).Path
}

func getCompletions(fpath string, compilationCtx *core.Context, line, column int32, session *jsonrpc.Session, fls afs.Filesystem) []compl.Completion {
	state, mod, _, err := inox_ns.PrepareLocalScript(inox_ns.ScriptPreparationArgs{
		Fpath:                     fpath,
		ParsingCompilationContext: compilationCtx,
		ParentContext:             nil,
		Out:                       io.Discard,
		IgnoreNonCriticalIssues:   true,
		AllowMissingEnvVars:       true,
		ScriptContextFileSystem:   fls,
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

func firstCharLspRange() defines.Range {
	return rangeToLspRange(parse.SourcePositionRange{
		StartLine:   1,
		StartColumn: 1,
		Span:        parse.NodeSpan{Start: 0, End: 1},
	})
}

func getLineColumn(pos defines.Position) (int32, int32) {
	line := int32(pos.Line + 1)
	column := int32(pos.Character + 1)
	return line, column
}
