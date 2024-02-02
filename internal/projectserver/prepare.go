package projectserver

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/inoxlang/inox/internal/core"
	fs_ns "github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/inoxlang/inox/internal/inoxconsts"
	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/projectserver/jsonrpc"
	"github.com/inoxlang/inox/internal/projectserver/logs"
	"github.com/inoxlang/inox/internal/projectserver/lsp/defines"
	"github.com/inoxlang/inox/internal/utils"
)

const (
	VERY_RECENT_ACTIVITY_DELTA = time.Second
	MAX_PREPARATION_DEPTH      = 2
)

type filePreparationParams struct {
	fpath   string
	session *jsonrpc.Session

	//if true and the state preparation failed then ok is false and results are nil.
	requiresState bool

	//if true and the file is not cached then ok is false and results are nil.
	//This setting has lower priority than forcePrepareIfNoVeryRecentActivity.
	requiresCache bool

	//preparation is attempted if true and the file is not cached
	//or the cache has not been updated/accessed very recently (VERY_RECENT_ACTIVITY_DELTA).
	forcePrepareIfNoVeryRecentActivity bool

	//if true the cache is not read but the resulting prepared file is cached.
	ignoreCache bool

	notifyUserAboutDbError bool

	_depth int //should not be by caller, it is used internally by prepareSourceFileInExtractionMode
}

type preparationResult struct {
	state                            *core.GlobalState
	module                           *core.Module
	chunk                            *parse.ParsedChunkSource
	failedToPrepareDBProvidingParent parse.Node
	cachedOrGotCache                 bool
}

// prepareSourceFileInExtractionMode prepares a module or includable-chunk file:
// - if requiresState is true & state failed to be created ok is false.
// - if the file at fpath is an includable-chunk the returned module is a fake module.
// - success is false if params.requiresCache is true and the file is not cached.
// The returned values SHOULD NOT BE MODIFIED.
func prepareSourceFileInExtractionMode(ctx *core.Context, params filePreparationParams) (prepResult preparationResult, success bool) {
	fpath := params.fpath
	session := params.session
	requiresState := params.requiresState
	project, _ := getProject(session)

	fls, new := getLspFilesystem(session)
	if !new {
		logs.Println("failed to get LSP filesystem")
		return
	}

	sessionData := getSessionData(params.session)
	var fileCache *preparedFileCacheEntry

	if params._depth > MAX_PREPARATION_DEPTH {
		session.Notify(NewShowMessage(defines.MessageTypeError, "maximum recursive preparation depth reached"))
		return
	}

	//we avoid locking the session data
	if sessionData.lock.TryLock() || sessionData.lock.TryLock() {
		if sessionData.preparedSourceFilesCache == nil {
			sessionData.preparedSourceFilesCache = newPreparedFileCache()
		}
		cache := sessionData.preparedSourceFilesCache
		sessionData.lock.Unlock()
		func() {
			fileCache, _ = cache.getOrCreate(fpath)
		}()

		//we lock the cache to pause parallel preparation of the same file
		fileCache.Lock()
		defer fileCache.Unlock()

		fileCache.acknowledgeAccess()
	} else if params.requiresCache {
		return
	}

	//check the cache
	if !params.ignoreCache && fileCache != nil {
		if fileCache.chunk != nil {
			logs.Println("cache hit for file", fpath)

			cachedChunk := fileCache.chunk
			cachedModule := fileCache.module
			cachedState := fileCache.state

			prepResult = preparationResult{
				state:            cachedState,
				module:           cachedModule,
				chunk:            cachedChunk,
				cachedOrGotCache: true,
			}
			success = true
			return

		} else if params.requiresCache && (!params.forcePrepareIfNoVeryRecentActivity ||
			time.Since(fileCache.LastUpdateOrInvalidation()) < VERY_RECENT_ACTIVITY_DELTA) {
			return
		} else {
			_ = 1
		}
	} else if params.requiresCache {
		return
	}

	chunk, err := core.ParseFileChunk(fpath, fls)

	if chunk == nil { //unrecoverable parsing error
		logs.Println("unrecoverable parsing error", err.Error())
		if params._depth == 0 {
			session.Notify(NewShowMessage(defines.MessageTypeError, err.Error()))
		}
		return
	}

	if chunk.Node.IncludableChunkDesc != nil {
		state, mod, includedChunk, err := core.PrepareExtractionModeIncludableChunkfile(core.IncludableChunkfilePreparationArgs{
			Fpath:                          fpath,
			ParsingContext:                 ctx,
			Out:                            io.Discard,
			LogOut:                         io.Discard,
			IncludedChunkContextFileSystem: fls,
		})

		if includedChunk == nil {
			logs.Println("unrecoverable parsing error", err.Error())
			session.Notify(NewShowMessage(defines.MessageTypeError, err.Error()))
			return
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

			return
		}

		//cache the results if the file was not modified during the preparation
		cached := false
		if fileCache != nil && !fileCache.clearIfSourceChanged() {
			cached = true
			fileCache.update(state, mod, includedChunk.ParsedChunkSource)
		}

		prepResult = preparationResult{
			state:            state,
			module:           mod,
			chunk:            includedChunk.ParsedChunkSource,
			cachedOrGotCache: cached,
		}
		success = true
		return
	} else {
		var parentCtx *core.Context

		if chunk.Node.Manifest != nil {
			//additional logic if the manifest refers to databases in another module
			if obj, ok := chunk.Node.Manifest.Object.(*parse.ObjectLiteral); ok {
				node, _ := obj.PropValue(core.MANIFEST_DATABASES_SECTION_NAME)

				if pathLiteral, ok := node.(*parse.AbsolutePathLiteral); ok {
					preparationResult, ok := prepareSourceFileInExtractionMode(ctx, filePreparationParams{
						fpath:                  pathLiteral.Value,
						session:                session,
						requiresState:          true,
						notifyUserAboutDbError: true,
						_depth:                 params._depth + 1,
					})
					if ok {
						parentCtx = preparationResult.state.Ctx
					} else {
						preparationResult.failedToPrepareDBProvidingParent = pathLiteral
					}
				}
			}
		}

		args := core.ModulePreparationArgs{
			Fpath:                     fpath,
			ParsingCompilationContext: ctx,

			//set if the module uses databases from another module.
			ParentContext:         parentCtx,
			ParentContextRequired: parentCtx != nil,
			DefaultLimits: []core.Limit{
				core.MustMakeNotAutoDepletingCountLimit(fs_ns.FS_READ_LIMIT_NAME, 10_000_000),
			},

			Out:                     io.Discard,
			DataExtractionMode:      true,
			ScriptContextFileSystem: fls,
			PreinitFilesystem:       fls,

			Project: project,
		}

		if strings.HasSuffix(fpath, inoxconsts.INOXLANG_SPEC_FILE_SUFFIX) {
			args.EnableTesting = true
		}

		state, mod, _, err := core.PrepareLocalModule(args)

		if mod == nil {
			logs.Println("unrecoverable parsing error", err.Error())
			session.Notify(NewShowMessage(defines.MessageTypeError, err.Error()))
			return
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

			return
		}

		if params.notifyUserAboutDbError && state != nil && state.FirstDatabaseOpeningError != nil {
			msg := fmt.Sprintf("failed to open at least one database in module %q: %s", params.fpath, state.FirstDatabaseOpeningError.Error())
			session.Notify(NewShowMessage(defines.MessageTypeWarning, msg))
		}

		cached := false
		//cache the results if the file was not modified during the preparation
		if fileCache != nil && !fileCache.clearIfSourceChanged() {
			cached = true
			fileCache.update(state, mod, nil)
		}

		prepResult = preparationResult{
			state:                            state,
			module:                           mod,
			chunk:                            mod.MainChunk,
			cachedOrGotCache:                 cached,
			failedToPrepareDBProvidingParent: prepResult.failedToPrepareDBProvidingParent,
		}
		success = true
		return
	}

}
