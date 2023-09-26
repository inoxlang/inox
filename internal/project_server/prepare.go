package project_server

import (
	"io"
	"sync"
	"sync/atomic"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/inox_ns"
	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/project_server/jsonrpc"
	"github.com/inoxlang/inox/internal/project_server/logs"
	"github.com/inoxlang/inox/internal/project_server/lsp/defines"
	"github.com/inoxlang/inox/internal/utils"
)

type preparedSourceFileCache struct {
	state  *core.GlobalState
	module *core.Module
	chunk  *parse.ParsedChunk
	lock   sync.Mutex

	clearBeforeNextAccess atomic.Bool
}

type filePreparationParams struct {
	fpath         string
	session       *jsonrpc.Session
	requiresState bool
	ignoreCache   bool //if true the cache is not read but the resulting prepared file is cached
}

// prepareSourceFileInExtractionMode prepares a module or includable-chunk file:
// - if requiresState is true & state failed to be created ok is false
// - if the file at fpath is an includable-chunk the returned module is a fake module
// The returned values SHOULD NOT BE MODIFIED.
func prepareSourceFileInExtractionMode(ctx *core.Context, params filePreparationParams) (*core.GlobalState, *core.Module, *parse.ParsedChunk, bool) {
	fpath := params.fpath
	session := params.session
	requiresState := params.requiresState
	project, _ := getProject(session)

	fls, ok := getLspFilesystem(session)
	if !ok {
		logs.Println("failed to get LSP filesystem")
		return nil, nil, nil, false
	}

	sessionData := getSessionData(params.session)
	var fileCache *preparedSourceFileCache

	//we avoid locking the session data
	if sessionData.lock.TryLock() || sessionData.lock.TryLock() {
		fileCache, ok = sessionData.preparedSourceFilesCache[fpath]
		if !ok {
			if sessionData.preparedSourceFilesCache == nil {
				sessionData.preparedSourceFilesCache = map[string]*preparedSourceFileCache{}
			}

			fileCache = &preparedSourceFileCache{}
			sessionData.preparedSourceFilesCache[fpath] = fileCache
		}
		sessionData.lock.Unlock()

		//we lock the cache to pause parallel preparation of the same file
		fileCache.lock.Lock()
		defer fileCache.lock.Unlock()

		if fileCache.clearBeforeNextAccess.CompareAndSwap(true, false) {
			fileCache.chunk = nil
			fileCache.module = nil
			fileCache.state = nil
		}
	}

	//check the cache
	if !params.ignoreCache && fileCache != nil && fileCache.chunk != nil {
		logs.Println("cache hit for file", fpath)

		cachedChunk := fileCache.chunk
		cachedModule := fileCache.module
		cachedState := fileCache.state

		if requiresState && (cachedState == nil || cachedState.SymbolicData == nil) {
			return nil, nil, nil, false
		}
		return cachedState, cachedModule, cachedChunk, true
	}

	chunk, err := core.ParseFileChunk(fpath, fls)

	if chunk == nil { //unrecoverable parsing error
		logs.Println("unrecoverable parsing error", err.Error())
		session.Notify(NewShowMessage(defines.MessageTypeError, err.Error()))
		return nil, nil, nil, false
	}

	if chunk.Node.IncludableChunkDesc != nil {
		state, mod, includedChunk, err := inox_ns.PrepareExtractionModeIncludableChunkfile(inox_ns.IncludableChunkfilePreparationArgs{
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

			if state != nil {
				//teardown
				go func() {
					defer utils.Recover()
					state.Ctx.CancelGracefully()
				}()
			}

			return nil, nil, nil, false
		}

		//cache the results if the file was not modified during the preparation
		if fileCache != nil {
			if fileCache.clearBeforeNextAccess.CompareAndSwap(true, false) {
				fileCache.state = nil
				fileCache.module = nil
				fileCache.chunk = nil
			} else {
				fileCache.state = state
				fileCache.module = mod
				fileCache.chunk = includedChunk.ParsedChunk
			}
		}

		return state, mod, includedChunk.ParsedChunk, true
	} else {
		var parentCtx *core.Context

		if chunk.Node.Manifest != nil {
			if obj, ok := chunk.Node.Manifest.Object.(*parse.ObjectLiteral); ok {
				node, _ := obj.PropValue(core.MANIFEST_DATABASES_SECTION_NAME)
				if pathLiteral, ok := node.(*parse.AbsolutePathLiteral); ok {
					state, _, _, ok := prepareSourceFileInExtractionMode(ctx, filePreparationParams{
						fpath:         pathLiteral.Value,
						session:       session,
						requiresState: true,
					})
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
			DataExtractionMode:      true,
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

			if state != nil {
				//teardown
				go func() {
					defer utils.Recover()
					state.Ctx.CancelGracefully()
				}()
			}

			return nil, nil, nil, false
		}

		//cache the results if the file was not modified during the preparation
		if fileCache != nil {
			if fileCache.clearBeforeNextAccess.CompareAndSwap(true, false) {
				fileCache.state = nil
				fileCache.module = nil
				fileCache.chunk = nil
			} else {
				logs.Println("update cache for file", fpath, "new length", len(mod.MainChunk.Source.Code()))
				fileCache.state = state
				fileCache.module = mod
				fileCache.chunk = mod.MainChunk
			}
		}

		return state, mod, mod.MainChunk, true
	}

}
