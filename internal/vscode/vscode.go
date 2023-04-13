package internal

import (
	"encoding/json"
	"fmt"
	"os"

	core "github.com/inox-project/inox/internal/core"

	"github.com/inox-project/inox/internal/utils"

	globals "github.com/inox-project/inox/internal/globals"

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
		const NO_DATA = `{"data": null}`

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

		fmt.Printf(`{"data": {"text":"%s"}}`+"\n", val.String())
		return
	default:
		fmt.Fprintf(os.Stderr, "unknown command `%s` for vsc subcommand\n", subCommand)
		os.Exit(1)
	}
}

type HoverRange [2]struct {
	Line   int32 //starts at 1
	Column int32 //start at 1
}
