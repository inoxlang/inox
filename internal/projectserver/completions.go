package projectserver

import (
	"io/fs"
	"strings"

	"github.com/inoxlang/inox/internal/afs"
	"github.com/inoxlang/inox/internal/codecompletion"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/http_ns/spec"
	"github.com/inoxlang/inox/internal/projectserver/jsonrpc"
	"github.com/inoxlang/inox/internal/utils"
)

// getCompletions gets the completions for a specific position in an Inox code file.
func getCompletions(fpath string, line, column int32, session *jsonrpc.Session) []codecompletion.Completion {
	sessionData := getLockedSessionData(session)

	fls := sessionData.filesystem
	if fls == nil {
		sessionData.lock.Unlock()
		return nil
	}

	serverAPI := sessionData.serverAPI
	sessionData.lock.Unlock()

	handlingCtx := session.Context().BoundChildWithOptions(core.BoundChildContextOptions{
		Filesystem: fls,
	})

	prepResult, ok := prepareSourceFileInExtractionMode(handlingCtx, filePreparationParams{
		fpath:         fpath,
		session:       session,
		requiresState: true,
	})
	if !ok {
		return nil
	}

	state := prepResult.state
	chunk := prepResult.chunk
	cachedOrHitCache := prepResult.cachedOrGotCache

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
	staticResourcePaths := getStaticResourcePaths(fls, "/static")
	var api *spec.API
	if serverAPI != nil {
		api = serverAPI.API()
	}

	return codecompletion.FindCompletions(codecompletion.SearchArgs{
		State:       core.NewTreeWalkStateWithGlobal(state),
		Chunk:       chunk,
		CursorIndex: int(pos),
		Mode:        codecompletion.LspCompletions,

		InputData: codecompletion.InputData{
			StaticFileURLPaths: staticResourcePaths,
			ServerAPI:          api,
		},
	})
}

func getStaticResourcePaths(fls afs.Filesystem, absStaticDir string) (paths []string) {
	//remove trailing slash
	if absStaticDir != "/" && absStaticDir[len(absStaticDir)-1] == '/' {
		absStaticDir = absStaticDir[:len(absStaticDir)-1]
	}

	core.WalkDirLow(fls, absStaticDir, func(path string, d fs.DirEntry, err error) error {
		paths = append(paths, strings.TrimPrefix(path, absStaticDir))
		return nil
	})

	return
}
