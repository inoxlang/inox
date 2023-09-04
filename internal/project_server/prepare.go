package project_server

import (
	"io"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/inox_ns"
	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/project_server/jsonrpc"
	"github.com/inoxlang/inox/internal/project_server/logs"
	"github.com/inoxlang/inox/internal/project_server/lsp/defines"
)

// prepareSourceFile prepares a module or includable-chunk file:
// - if requiresState is true & state failed to be created ok is false
// - if the file at fpath is an includable-chunk the returned module is a fake module
func prepareSourceFile(fpath string, ctx *core.Context, session *jsonrpc.Session, requiresState bool) (*core.GlobalState, *core.Module, *parse.ParsedChunk, bool) {
	project, _ := getProject(session)

	fls, ok := getLspFilesystem(session)
	if !ok {
		logs.Println("failed to get LSP filesystem")
		return nil, nil, nil, false
	}

	chunk, err := core.ParseFileChunk(fpath, fls)

	if chunk == nil { //unrecoverable parsing error
		logs.Println("unrecoverable parsing error", err.Error())
		session.Notify(NewShowMessage(defines.MessageTypeError, err.Error()))
		return nil, nil, nil, false
	}

	if chunk.Node.IncludableChunkDesc != nil {
		state, mod, includedChunk, err := inox_ns.PrepareDevModeIncludableChunkfile(inox_ns.IncludableChunkfilePreparationArgs{
			Fpath:                          fpath,
			ParsingContext:                 ctx,
			Out:                            io.Discard,
			LogOut:                         io.Discard,
			IncludedChunkContextFileSystem: fls,
		})

		if includedChunk == nil {
			logs.Println("unrecoverable parsing error", err.Error())
			session.Notify(NewShowMessage(defines.MessageTypeError, err.Error()))
			return nil, nil, nil, false
		}

		if requiresState && (state == nil || state.SymbolicData == nil) {
			logs.Println("failed to prepare includable-chunk", err.Error())
			return nil, nil, nil, false
		}

		return state, mod, includedChunk.ParsedChunk, true
	} else {
		var parentCtx *core.Context

		if chunk.Node.Manifest != nil {
			if obj, ok := chunk.Node.Manifest.Object.(*parse.ObjectLiteral); ok {
				node, _ := obj.PropValue(core.MANIFEST_DATABASES_SECTION_NAME)
				if pathLiteral, ok := node.(*parse.AbsolutePathLiteral); ok {
					state, _, _, ok := prepareSourceFile(pathLiteral.Value, ctx, session, true)
					if ok {
						parentCtx = state.Ctx
					}
				}
			}
		}

		state, mod, _, err := inox_ns.PrepareLocalScript(inox_ns.ScriptPreparationArgs{
			Fpath:                     fpath,
			ParsingCompilationContext: ctx,

			//set if the module uses databases from another module.
			ParentContext:         parentCtx,
			ParentContextRequired: parentCtx != nil,

			Out:                     io.Discard,
			DevMode:                 true,
			AllowMissingEnvVars:     true,
			ScriptContextFileSystem: fls,
			PreinitFilesystem:       fls,

			Project: project,
		})

		if mod == nil {
			logs.Println("unrecoverable parsing error", err.Error())
			session.Notify(NewShowMessage(defines.MessageTypeError, err.Error()))
			return nil, nil, nil, false
		}

		if requiresState && (state == nil || state.SymbolicData == nil) {
			logs.Println("failed to prepare module", err.Error())
			return nil, nil, nil, false
		}

		return state, mod, mod.MainChunk, true
	}

}
