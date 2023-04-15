package internal

import (
	"context"
	"log"
	"os"

	"github.com/TobiasYin/go-lsp/logs"
	"github.com/TobiasYin/go-lsp/lsp"
	"github.com/TobiasYin/go-lsp/lsp/defines"
	core "github.com/inox-project/inox/internal/core"

	"github.com/inox-project/inox/internal/utils"

	globals "github.com/inox-project/inox/internal/globals"
	compl "github.com/inox-project/inox/internal/globals/completion"

	_ "net/http/pprof"
	"net/url"
)

func StartLSPServer() {

	f, err := os.OpenFile(".debug.txt", os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o600)
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
	})

	compilationCtx := core.NewContext(core.ContextConfig{
		Permissions: []core.Permission{
			//TODO: change path pattern
			core.FilesystemPermission{Kind_: core.ReadPerm, Entity: core.PathPattern("/...")},
		},
	})
	core.NewGlobalState(compilationCtx)

	server.OnHover(func(ctx context.Context, req *defines.HoverParams) (result *defines.Hover, err error) {
		logs.Println(req)

		fpath := getFilePath(req.TextDocument)
		line := int32(req.Position.Line + 1)
		column := int32(req.Position.Character + 1)

		state, mod, _ := globals.PrepareLocalScript(globals.ScriptPreparationArgs{
			Fpath:                     fpath,
			PassedArgs:                []string{},
			ParsingCompilationContext: compilationCtx,
			ParentContext:             nil,
			Out:                       os.Stdout,
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

		val, ok := state.SymbolicData.GetNodeValue(foundNode)
		if !ok {
			logs.Println("no data")
			return &defines.Hover{}, nil
		}

		return &defines.Hover{
			Contents: defines.MarkupContent{
				Kind:  defines.MarkupKindPlainText,
				Value: val.String(),
			},
		}, nil
	})

	server.OnCompletion(func(ctx context.Context, req *defines.CompletionParams) (result *[]defines.CompletionItem, err error) {
		logs.Println(req)
		textKind := defines.CompletionItemKindText

		fpath := getFilePath(req.TextDocument)
		line := int32(req.Position.Line + 1)
		column := int32(req.Position.Character + 1)

		state, mod, _ := globals.PrepareLocalScript(globals.ScriptPreparationArgs{
			Fpath:                     fpath,
			PassedArgs:                []string{},
			ParsingCompilationContext: compilationCtx,
			ParentContext:             nil,
			Out:                       os.Stdout,
		})

		chunk := mod.MainChunk
		pos := chunk.GetLineColumnPosition(line, column)

		completions := compl.FindCompletions(core.NewTreeWalkStateWithGlobal(state), chunk, int(pos))

		lspCompletions := utils.MapSlice(completions, func(completion compl.Completion) defines.CompletionItem {
			range_ := completion.ReplacedRange

			return defines.CompletionItem{
				Label: completion.Value,
				Kind:  &textKind,
				TextEdit: defines.TextEdit{
					Range: defines.Range{
						Start: defines.Position{
							Line:      uint(range_.StartLine) - 1,
							Character: uint(range_.StartColumn - 1),
						},
						End: defines.Position{
							Line:      uint(range_.StartLine) - 1,
							Character: uint(range_.StartColumn - 1 + range_.Span.End - range_.Span.Start),
						},
					},
				},
				InsertText: &completion.Value,
			}
		})

		return &lspCompletions, nil
	})

	server.Run()
}

func getFilePath(doc defines.TextDocumentIdentifier) string {
	return utils.Must(url.Parse(string(doc.Uri))).Path
}
