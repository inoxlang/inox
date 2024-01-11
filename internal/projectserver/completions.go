package projectserver

import (
	"github.com/inoxlang/inox/internal/codecompletion"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/projectserver/jsonrpc"
	"github.com/inoxlang/inox/internal/utils"
)

func getCompletions(fpath string, line, column int32, session *jsonrpc.Session) []codecompletion.Completion {
	fls, ok := getLspFilesystem(session)
	if !ok {
		return nil
	}

	handlingCtx := session.Context().BoundChildWithOptions(core.BoundChildContextOptions{
		Filesystem: fls,
	})

	state, _, chunk, cachedOrHitCache, ok := prepareSourceFileInExtractionMode(handlingCtx, filePreparationParams{
		fpath:         fpath,
		session:       session,
		requiresState: true,
	})
	if !ok {
		return nil
	}

	if !cachedOrHitCache && state != nil {
		//teardown in separate goroutine to return quickly
		defer func() {
			go func() {
				defer utils.Recover()
				state.Ctx.CancelGracefully()
			}()
		}()
	}

	if state == nil {
		return nil
	}

	pos := chunk.GetLineColumnPosition(line, column)

	return codecompletion.FindCompletions(codecompletion.SearchArgs{
		State:       core.NewTreeWalkStateWithGlobal(state),
		Chunk:       chunk,
		CursorIndex: int(pos),
		Mode:        codecompletion.LspCompletions,
	})
}
