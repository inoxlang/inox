package internal

import (
	"context"
	"encoding/json"
	"fmt"
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

		fpath := utils.Must(url.Parse(string(req.TextDocument.Uri))).Path
		line := req.Position.Line + 1
		column := req.Position.Character + 1

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

		span := mod.MainChunk.GetLineColumnSingeCharSpan(int32(line), int32(column))
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

	var hello = "Hello"

	server.OnCompletion(func(ctx context.Context, req *defines.CompletionParams) (result *[]defines.CompletionItem, err error) {
		logs.Println(req)
		d := defines.CompletionItemKindText
		return &[]defines.CompletionItem{{
			Label:      "code",
			Kind:       &d,
			InsertText: &hello,
		}}, nil
	})

	server.Run()
}

func HandleVscCommand(fpath string, dir string, subCommand string, jsonData string) {

	compilationCtx := core.NewContext(core.ContextConfig{
		Permissions: []core.Permission{
			//TODO: change path pattern
			core.FilesystemPermission{Kind_: core.ReadPerm, Entity: core.PathPattern("/...")},
		},
	})
	core.NewGlobalState(compilationCtx)

	switch subCommand {
	case "get-hover-data":
		const NO_DATA = `{}`

		var hoverRange HoverRange
		if err := json.Unmarshal(utils.StringAsBytes(jsonData), &hoverRange); err != nil {
			fmt.Println(err)
			return
		}
		return
	case "get-completions":
		var lineCol LineColumn
		if err := json.Unmarshal(utils.StringAsBytes(jsonData), &lineCol); err != nil {
			fmt.Println(err)
			return
		}

		state, mod, _ := globals.PrepareLocalScript(globals.ScriptPreparationArgs{
			Fpath:                     fpath,
			PassedArgs:                []string{},
			ParsingCompilationContext: compilationCtx,
			ParentContext:             nil,
			Out:                       os.Stdout,
		})

		chunk := mod.MainChunk
		pos := chunk.GetLineColumnPosition(lineCol.Line, lineCol.Column)

		completions := compl.FindCompletions(core.NewTreeWalkStateWithGlobal(state), chunk, int(pos))
		data := CompletionData{Completions: utils.EmptySliceIfNil(completions)}
		dataJSON := utils.Must(json.Marshal(data))

		fmt.Println(utils.BytesAsString(dataJSON))
		return
	default:
		fmt.Fprintf(os.Stderr, "unknown command `%s` for vsc subcommand\n", subCommand)
		os.Exit(1)
	}
}

type LineColumn struct {
	Line   int32 //starts at 1
	Column int32 //start at 1
}

type HoverRange [2]LineColumn

type CompletionData struct {
	Completions []compl.Completion `json:"completions"`
}
