package internal

import (
	"encoding/json"
	"fmt"
	"os"

	core "github.com/inox-project/inox/internal/core"

	"github.com/inox-project/inox/internal/utils"

	globals "github.com/inox-project/inox/internal/globals"
	compl "github.com/inox-project/inox/internal/globals/completion"

	_ "net/http/pprof"
)

func HandleVscCommand(fpath string, dir string, subCommand string, jsonData string) {

	compilationCtx := core.NewContext(core.ContextConfig{
		Permissions: []core.Permission{
			core.FilesystemPermission{Kind_: core.ReadPerm, Entity: core.PathPattern(dir + "...")},
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

		state, mod, _ := globals.PrepareLocalScript(globals.ScriptPreparationArgs{
			Fpath:                     fpath,
			PassedArgs:                []string{},
			ParsingCompilationContext: compilationCtx,
			ParentContext:             nil,
			Out:                       os.Stdout,
		})

		if state == nil || state.SymbolicData == nil {
			fmt.Println(NO_DATA)
			return
		}

		line := hoverRange[0].Line
		column := hoverRange[0].Column

		span := mod.MainChunk.GetLineColumnSingeCharSpan(line, column)
		foundNode, ok := mod.MainChunk.GetNodeAtSpan(span)

		if !ok || foundNode == nil {
			fmt.Println(NO_DATA)
			return
		}

		val, ok := state.SymbolicData.GetNodeValue(foundNode)
		if !ok {
			fmt.Println(NO_DATA)
			return
		}

		data := HoverData{Text: val.String()}
		dataJSON := utils.Must(json.Marshal(data))

		fmt.Println(utils.BytesAsString(dataJSON))
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

type HoverData struct {
	Text string `json:"text"`
}

type CompletionData struct {
	Completions []compl.Completion `json:"completions"`
}
