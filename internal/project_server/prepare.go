package project_server

import (
	"io"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/inox_ns"
	"github.com/inoxlang/inox/internal/project_server/jsonrpc"
	"github.com/inoxlang/inox/internal/project_server/logs"
	"github.com/inoxlang/inox/internal/project_server/lsp/defines"
)

func prepareSourceFile(fpath string, ctx *core.Context, session *jsonrpc.Session) (*core.GlobalState, *core.Module, bool) {
	fls, ok := getLspFilesystem(session)
	if !ok {
		logs.Println("failed to get LSP filesystem")
		return nil, nil, false
	}

	state, mod, _, err := inox_ns.PrepareLocalScript(inox_ns.ScriptPreparationArgs{
		Fpath:                     fpath,
		ParsingCompilationContext: ctx,
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
		return nil, nil, false
	}

	if state == nil || state.SymbolicData == nil {
		logs.Println("failed to prepare script", err.Error())
		return nil, nil, false
	}

	return state, mod, true
}
