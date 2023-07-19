package project_server

import (
	"io"

	core "github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/compl"
	"github.com/inoxlang/inox/internal/globals/inox_ns"
	"github.com/inoxlang/inox/internal/project_server/jsonrpc"
	"github.com/inoxlang/inox/internal/project_server/logs"
	"github.com/inoxlang/inox/internal/project_server/lsp/defines"
)

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
