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
	"github.com/inoxlang/inox/internal/project"
	"github.com/inoxlang/inox/internal/projectserver/jsonrpc"
	"github.com/inoxlang/inox/internal/projectserver/lsp/defines"
	"github.com/inoxlang/inox/internal/utils"
)

const (
	VERY_RECENT_ACTIVITY_DELTA = time.Second
	MAX_PREPARATION_DEPTH      = 2

	SINGLE_FILE_PARSING_TIMEOUT = 50 * time.Millisecond
)

type filePreparationParams struct {
	fpath      string
	rpcSession *jsonrpc.Session

	//If true and the state preparation failed then ok is false and results are nil.
	requiresState bool

	//If true and the file is not cached then ok is false and results are nil.
	//This setting has lower priority than forcePrepareIfNoVeryRecentActivity and alwaysForcePrepare.
	requiresCache bool

	//Forces the preparation if the prepared file is not cached AND the cache has not been updated/accessed very recently (VERY_RECENT_ACTIVITY_DELTA).
	forcePrepareIfNoVeryRecentActivity bool

	//Always forces the preparation if the prepared file is not cached.
	alwaysForcePrepare bool

	//If true the cache is not read but the resulting prepared file is cached.
	ignoreCache bool

	//Defaults to SINGLE_FILE_PARSING_TIMEOUT.
	singleFileParsingTimeout time.Duration

	notifyUserAboutDbError bool

	_depth int //The field should not be set by the caller, it is used internally by prepareSourceFileInExtractionMode

	//The following fields are passed directly to prepareSourceFileInExtractionMode so that it does not have to lock the session to retrieve them.

	memberAuthToken string
	project         *project.Project
	lspFilesystem   *Filesystem
	inoxChunkCache  *parse.ChunkCache
}

type preparationResult struct {
	state                            *core.GlobalState
	module                           *core.Module
	chunk                            *parse.ParsedChunkSource
	failedToPrepareDBProvidingParent parse.Node
	cachedOrGotCache                 bool
}

// prepareSourceFileInExtractionMode prepares a module or includable-file file:
// - if requiresState is true & state failed to be created ok is false.
// - if the file at fpath is an includable-file the returned module is a fake module.
// - success is false if params.requiresCache is true and the file is not cached.
// The returned values SHOULD NOT BE MODIFIED.
func prepareSourceFileInExtractionMode(ctx *core.Context, params filePreparationParams) (prepResult preparationResult, success bool) {
	fpath := params.fpath
	rpcSession := params.rpcSession
	requiresState := params.requiresState
	project := params.project
	lspFilesystem := params.lspFilesystem

	singleFileParsingTimeout := utils.DefaultIfZero(params.singleFileParsingTimeout, SINGLE_FILE_PARSING_TIMEOUT)

	session := getCreateProjectSession(rpcSession)
	var fileCache *preparedFileCacheEntry

	if params._depth > MAX_PREPARATION_DEPTH {
		rpcSession.Notify(NewShowMessage(defines.MessageTypeError, "maximum recursive preparation depth reached"))
		return
	}

	//Try to lock the session to get the cache.
	if session.lock.TryLock() {
		//-------------------------------------------------------------
		if session.preparedSourceFilesCache == nil {
			session.preparedSourceFilesCache = newPreparedFileCache(rpcSession.Logger())
		}
		cache := session.preparedSourceFilesCache
		session.lock.Unlock()
		//-------------------------------------------------------------

		func() {
			fileCache, _ = cache.getOrCreate(fpath)
		}()

		//Lock the cache entry to prevent parallel preparation of the same file.
		fileCache.Lock()
		defer fileCache.Unlock()

		fileCache.acknowledgeAccess()
	} else if params.requiresCache {
		//Failure
		return
	}

	//Check the cache entry.
	if !params.ignoreCache && fileCache != nil {
		if fileCache.chunk != nil {
			rpcSession.LoggerPrintln("cache hit for file", fpath)

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

		} else if params.requiresCache && !params.alwaysForcePrepare &&
			(!params.forcePrepareIfNoVeryRecentActivity || time.Since(fileCache.LastUpdateOrInvalidation()) < VERY_RECENT_ACTIVITY_DELTA) {
			return
		}
		//else: prepare
	} else if params.requiresCache {
		return
	}

	chunk, err := core.ParseFileChunk(fpath, lspFilesystem, parse.ParserOptions{
		Timeout: singleFileParsingTimeout,
	})

	if chunk == nil { //unrecoverable parsing error
		rpcSession.LoggerPrintln("unrecoverable parsing error", err.Error())
		if params._depth == 0 {
			rpcSession.Notify(NewShowMessage(defines.MessageTypeError, err.Error()))
		}
		return
	}

	if chunk.Node.IncludableChunkDesc != nil { //prepare includable file
		state, mod, includedChunk, err := core.PrepareExtractionModeIncludableFile(core.IncludableFilePreparationArgs{
			Fpath:                    fpath,
			ParsingContext:           ctx,
			SingleFileParsingTimeout: singleFileParsingTimeout,
			InoxChunkCache:           params.inoxChunkCache,

			Out:                            io.Discard,
			LogOut:                         io.Discard,
			IncludedChunkContextFileSystem: lspFilesystem,
		})

		if includedChunk == nil {
			rpcSession.LoggerPrintln("unrecoverable parsing error", err.Error())
			rpcSession.Notify(NewShowMessage(defines.MessageTypeError, err.Error()))
			return
		}

		if requiresState && (state == nil || state.SymbolicData == nil) {
			rpcSession.LoggerPrintln("failed to prepare includable-file", err.Error())

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
	} else { //prepare module
		var parentCtx *core.Context

		if chunk.Node.Manifest != nil {
			//Additional logic if the manifest refers to databases in another module.
			if obj, ok := chunk.Node.Manifest.Object.(*parse.ObjectLiteral); ok {
				node, _ := obj.PropValue(inoxconsts.MANIFEST_DATABASES_SECTION_NAME)

				if pathLiteral, ok := node.(*parse.AbsolutePathLiteral); ok {
					preparationResult, ok := prepareSourceFileInExtractionMode(ctx, filePreparationParams{
						fpath:                    pathLiteral.Value,
						requiresState:            true,
						notifyUserAboutDbError:   true,
						_depth:                   params._depth + 1,
						singleFileParsingTimeout: singleFileParsingTimeout,

						rpcSession:      rpcSession,
						project:         project,
						lspFilesystem:   lspFilesystem,
						inoxChunkCache:  params.inoxChunkCache,
						memberAuthToken: params.memberAuthToken,
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
			SingleFileParsingTimeout:  singleFileParsingTimeout,
			InoxChunkCache:            params.inoxChunkCache,

			//set if the module uses databases from another module.
			ParentContext:         parentCtx,
			ParentContextRequired: parentCtx != nil,
			DefaultLimits: []core.Limit{
				core.MustMakeNotAutoDepletingCountLimit(fs_ns.FS_READ_LIMIT_NAME, 10_000_000),
			},

			Out:                     io.Discard,
			DataExtractionMode:      true,
			ScriptContextFileSystem: lspFilesystem,
			PreinitFilesystem:       lspFilesystem,

			Project:         project,
			MemberAuthToken: params.memberAuthToken,
		}

		if strings.HasSuffix(fpath, inoxconsts.INOXLANG_SPEC_FILE_SUFFIX) {
			args.EnableTesting = true
		}

		state, mod, _, err := core.PrepareLocalModule(args)

		if mod == nil {
			rpcSession.LoggerPrintln("unrecoverable parsing error", err.Error())
			rpcSession.Notify(NewShowMessage(defines.MessageTypeError, err.Error()))
			return
		}

		if requiresState && (state == nil || state.SymbolicData == nil) {
			rpcSession.LoggerPrintln("failed to prepare module", err.Error())

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
			rpcSession.Notify(NewShowMessage(defines.MessageTypeWarning, msg))
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
