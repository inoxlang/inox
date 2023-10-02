package project_server

import (
	"github.com/inoxlang/inox/internal/compl"
	core "github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/project_server/jsonrpc"
	"github.com/inoxlang/inox/internal/utils"
)

func getCompletions(fpath string, line, column int32, session *jsonrpc.Session) []compl.Completion {
	fls, ok := getLspFilesystem(session)
	if !ok {
		return nil
	}

	handlingCtx := session.Context().BoundChildWithOptions(core.BoundChildContextOptions{
		Filesystem: fls,
	})

	state, _, chunk, ok := prepareSourceFileInExtractionMode(handlingCtx, filePreparationParams{
		fpath:         fpath,
		session:       session,
		requiresState: true,
	})
	if !ok {
		return nil
	}

	//teardown
	defer func() {
		go func() {
			defer utils.Recover()
			state.Ctx.CancelGracefully()
		}()
	}()

	pos := chunk.GetLineColumnPosition(line, column)

	return compl.FindCompletions(compl.CompletionSearchArgs{
		State:       core.NewTreeWalkStateWithGlobal(state),
		Chunk:       chunk,
		CursorIndex: int(pos),
		Mode:        compl.LspCompletions,
	})
}
