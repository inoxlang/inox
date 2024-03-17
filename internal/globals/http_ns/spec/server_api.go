package spec

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"runtime/debug"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/inoxlang/inox/internal/afs"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/inoxlang/inox/internal/inoxconsts"
	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/utils"
)

const (
	FS_ROUTING_BODY_PARAM   = "_body"
	FS_ROUTING_METHOD_PARAM = "_method"
	FS_ROUTING_INDEX_MODULE = "index" + inoxconsts.INOXLANG_FILE_EXTENSION

	SINGLE_FILE_PARSING_TIMEOUT = 50 * time.Millisecond
)

var (
	ErrUnexpectedBodyParamsInGETHandler            = errors.New("unexpected request body parmameters in GET handler")
	ErrUnexpectedBodyParamsInOPTIONSHandler        = errors.New("unexpected request body parmameters in OPTIONS handler")
	ErrUnexpectedBodyParamsInMethodAgnosticHandler = errors.New("unexpected request body parmameters in method-agnostic handler")
)

type ServerApiResolutionConfig struct {
	DynamicDir              string
	IgnoreModulesWithErrors bool
}

func GetFSRoutingServerAPI(ctx *core.Context, config ServerApiResolutionConfig) (*API, error) {
	preparedModuleCache := map[string]*core.GlobalState{}
	defer func() {
		for _, state := range preparedModuleCache {
			state.Ctx.CancelGracefully()
		}
	}()

	endpoints := make(map[string]*ApiEndpoint)

	if config.DynamicDir != "" {
		state := &fsRoutingAPIConstructionState{
			ctx:             ctx,
			tempModuleCache: preparedModuleCache,
			pState:          &parallelState{endpointLocks: map[*ApiEndpoint]*sync.Mutex{}},
			endpoints:       endpoints,
			fls:             ctx.GetFileSystem(),
		}
		err := addFsDirEndpoints(config.DynamicDir, "/", state)
		if err != nil {
			return nil, err
		}
	}

	return NewAPI(endpoints)
}

type fsRoutingAPIConstructionState struct {
	ctx             *core.Context
	config          ServerApiResolutionConfig
	endpoints       map[string]*ApiEndpoint
	tempModuleCache map[string]*core.GlobalState
	pState          *parallelState
	fls             afs.Filesystem
}

type parallelState struct {
	errors     []error
	operations []*ApiOperation //same length as .errors
	endpoints  []*ApiEndpoint  //same length as .errors

	endpointLocks map[*ApiEndpoint]*sync.Mutex

	lock sync.Mutex
}

// addFsDirEndpoints recursively add the endpoints defined in dir and its subdirectories.
func addFsDirEndpoints(dir string, urlDirPath string, state *fsRoutingAPIConstructionState) error {
	pstate := state.pState

	//Normalize the directory and the URL directory.

	dir = core.AppendTrailingSlashIfNotPresent(dir)
	urlDirPath = core.AppendTrailingSlashIfNotPresent(urlDirPath)

	//Recursively handle entries.

	err := addFsDirEndpointsOfEntries(dir, urlDirPath, state)

	if err != nil {
		return err
	}

	//Handle errors
	for i, err := range pstate.errors {
		if state.config.IgnoreModulesWithErrors {
			endpoint := pstate.endpoints[i]
			operation := pstate.operations[i]

			if endpoint == nil || operation == nil {
				continue
			}

			if len(endpoint.operations) == 1 {
				delete(state.endpoints, endpoint.path)
			} else {
				endpoint.operations = slices.DeleteFunc(endpoint.operations, func(op ApiOperation) bool {
					return op.httpMethod == operation.httpMethod
				})
			}

			continue
		} else {
			return err
		}
	}

	//Update catch-all endpoints.
	for _, endpt := range state.endpoints {
		pstate.lock.Lock()
		endpointLock := pstate.endpointLocks[endpt]
		pstate.lock.Unlock()

		func() {
			//We need to lock because the same endpoint can be mutated by a goroutine created by the caller.
			//This also ensures that all goroutines created to prepare the handler modules are finished.
			endpointLock.Lock()
			defer endpointLock.Unlock()

			if len(endpt.operations) == 1 && endpt.operations[0].httpMethod == "" {
				{

					operation := endpt.operations[0]
					endpt.operations = nil
					endpt.hasMethodAgnosticHandler = true
					endpt.methodAgnosticHandler = operation.handlerModule
				}
			}
		}()
	}

	return nil
}

func addFsDirEndpointsOfEntries(dir, urlDirPath string, state *fsRoutingAPIConstructionState) error {

	urlDirPathNoTrailingSlash := strings.TrimSuffix(urlDirPath, "/")
	if urlDirPath == "/" {
		urlDirPathNoTrailingSlash = "/"
	}

	ctx := state.ctx
	dirBasename := filepath.Base(dir)

	entries, err := state.fls.ReadDir(dir)

	if err != nil {
		return err
	}

	wg := new(sync.WaitGroup) //This wait group is waited for in 2 different places.

	//TODO: prevent blocking + add a timeout (kill context)
	defer wg.Wait()

	for _, entry := range entries {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		entryName := entry.Name()
		absEntryPath := filepath.Join(dir, entryName)

		//If the entry is a directory we recursively add the endpoints defined inside it.
		if entry.IsDir() {
			subDir := absEntryPath + "/"
			urlSubDir := ""
			if entryName[0] == ':' {
				urlSubDir = filepath.Join(urlDirPath, "{"+entryName[1:]+"}") + "/"
			} else {
				urlSubDir = filepath.Join(urlDirPath, entryName) + "/"
			}

			//wg.Wait()

			err := addFsDirEndpoints(subDir, urlSubDir, state)
			if err != nil {
				return err
			}
			continue
		}

		//Ignore non-Inox files and .spec.ix files.
		if !strings.HasSuffix(entryName, inoxconsts.INOXLANG_FILE_EXTENSION) || strings.HasSuffix(entryName, inoxconsts.INOXLANG_SPEC_FILE_SUFFIX) {
			continue
		}

		entryNameNoExt := strings.TrimSuffix(entryName, inoxconsts.INOXLANG_FILE_EXTENSION)

		//Determine the endpoint path and method by 'parsing' the entry name.
		var endpointPath string
		var method string //if empty the handler module supports several methods
		returnErrIfNotModule := true

		if slices.Contains(FS_ROUTING_METHODS, entryNameNoExt) { //GET.ix, POST.ix, ...
			//add operation
			method = entryNameNoExt
			endpointPath = urlDirPathNoTrailingSlash
		} else {
			beforeName, name, ok := strings.Cut(entryNameNoExt, "-")

			if ok && slices.Contains(FS_ROUTING_METHODS, beforeName) { //POST-... , GET-...
				method = beforeName

				if name == dirBasename { //example: POST-users in a 'users' directory
					endpointPath = urlDirPathNoTrailingSlash
				} else {
					endpointPath = filepath.Join(urlDirPath, name)
				}

			} else if entryName == FS_ROUTING_INDEX_MODULE { //index.ix
				method = "GET"
				endpointPath = urlDirPathNoTrailingSlash
			} else { //example: about.ix
				method = "GET"
				endpointPath = filepath.Join(urlDirPath, entryNameNoExt)
				returnErrIfNotModule = false
			}
		}

		//Remove trailing slash.
		if endpointPath != "/" {
			endpointPath = strings.TrimSuffix(endpointPath, "/")
		}

		//Determine if the file is an Inox module.
		chunk, err := core.ParseFileChunk(absEntryPath, state.fls, parse.ParserOptions{
			Timeout: SINGLE_FILE_PARSING_TIMEOUT,
		})

		if err != nil {
			if state.config.IgnoreModulesWithErrors {
				continue
			}
			return fmt.Errorf("failed to parse %q: %w", absEntryPath, err)
		}

		if chunk.Node.Manifest == nil { //not a module
			if returnErrIfNotModule {
				return fmt.Errorf("%q is not a module", absEntryPath)
			}
			continue
		}

		err = addOperationFsRouting(operationAdditionParams{
			endpointPath: endpointPath,
			absEntryPath: absEntryPath,
			chunk:        chunk,
			method:       method,
			wg:           wg,
			state:        state,
		})

		if err != nil {
			return err
		}
	}

	return nil
}

type operationAdditionParams struct {
	endpointPath, absEntryPath, method string
	chunk                              *parse.ParsedChunkSource
	wg                                 *sync.WaitGroup
	state                              *fsRoutingAPIConstructionState
}

func addOperationFsRouting(params operationAdditionParams) error {
	endpointPath := params.endpointPath
	absEntryPath := params.absEntryPath
	chunk := params.chunk

	method := params.method
	wg := params.wg

	constructionState := params.state
	pstate := constructionState.pState
	constructionInitiator, _ := constructionState.ctx.GetState()

	endpt := constructionState.endpoints[endpointPath]
	if endpt == nil && endpointPath == "/" {
		endpt = &ApiEndpoint{
			path: "/",
		}
		constructionState.endpoints[endpointPath] = endpt
	} else if endpt == nil {
		//Add endpoint into the API.
		endpt = &ApiEndpoint{
			path: endpointPath,
		}
		constructionState.endpoints[endpointPath] = endpt
		if endpointPath == "" || endpointPath[0] != '/' {
			return fmt.Errorf("invalid endpoint path %q", endpointPath)
		}
	}

	//We need to lock the endpoint because the same endpoint can be mutated by a goroutine created by the caller.
	pstate.lock.Lock()
	endpointLock, ok := pstate.endpointLocks[endpt]
	if !ok {
		endpointLock = new(sync.Mutex)
		pstate.endpointLocks[endpt] = endpointLock
	}
	pstate.lock.Unlock()

	//The endpoint is locked, and $endpointLock is passed to the goroutine preparing the module
	//so that it can unlock the endpoint when it is done.
	endpointLock.Lock()

	//Check the same operation is not already defined.
	for _, op := range endpt.operations {
		if op.httpMethod == method || method == "" {
			if op.handlerModule != nil {
				endpointLock.Unlock()
				return fmt.Errorf(
					"operation %s %q is already implemented by the module %q; unexpected module %q",
					op.httpMethod, endpointPath, op.handlerModule.ModuleName(), absEntryPath)
			}
			endpointLock.Unlock()

			return fmt.Errorf(

				"operation %s %q is already implemented; unexpected module %q",
				op.httpMethod, endpointPath, absEntryPath)
		}
	}

	endpt.operations = append(endpt.operations, ApiOperation{
		httpMethod: method,
	})

	operation := &endpt.operations[len(endpt.operations)-1]

	wg.Add(1)

	//Create a goroutine that will prepare the module and determine the parameters (schema) of the operation.

	go func() {
		defer wg.Done()
		defer endpointLock.Unlock()

		errorIndex := -1

		defer func() {
			e := recover()
			if e != nil {
				err := utils.ConvertPanicValueToError(e)
				err = fmt.Errorf("%w: %s", err, debug.Stack())

				if errorIndex >= 0 {
					pstate.errors[errorIndex] = err
				} else {
					pstate.lock.Lock()
					defer pstate.lock.Unlock()
					pstate.errors = append(pstate.errors, err)
					pstate.operations = append(pstate.operations, nil)
					pstate.endpoints = append(pstate.endpoints, nil)
				}
			}
		}()

		goroutineCtx := constructionState.ctx.BoundChild()
		defer goroutineCtx.CancelGracefully()

		manifestObj := chunk.Node.Manifest.Object.(*parse.ObjectLiteral)
		dbSection, _ := manifestObj.PropValue(core.MANIFEST_DATABASES_SECTION_NAME)

		var dbProviderContext *core.Context = constructionState.ctx

		//If the databases are defined in another module we retrieve this module.
		if path, ok := dbSection.(*parse.AbsolutePathLiteral); ok {
			if cache, ok := constructionState.tempModuleCache[path.Value]; ok {
				dbProviderContext = cache.Ctx

				//If the condition is false (the construction initiator is the module providing the database),
				//there is nothing to do as the dbProviderContext is already set to its context.
			} else if constructionInitiator.Module.Name() != path.Value {

				modState, _, _, err := core.PrepareLocalModule(core.ModulePreparationArgs{
					Fpath:                     path.Value,
					ParsingCompilationContext: goroutineCtx,
					SingleFileParsingTimeout:  SINGLE_FILE_PARSING_TIMEOUT,

					ParentContext:         goroutineCtx,
					ParentContextRequired: true,
					DefaultLimits: []core.Limit{
						core.MustMakeNotAutoDepletingCountLimit(fs_ns.FS_READ_LIMIT_NAME, 10_000_000),
					},

					Out:                     io.Discard,
					DataExtractionMode:      true,
					ScriptContextFileSystem: constructionState.fls,
					PreinitFilesystem:       constructionState.fls,
				})

				if err != nil {
					pstate.lock.Lock()
					defer pstate.lock.Unlock()

					errorIndex = len(pstate.errors)
					pstate.errors = append(pstate.errors, err)
					pstate.operations = append(pstate.operations, operation)
					pstate.endpoints = append(pstate.endpoints, endpt)
					return
				}

				constructionState.tempModuleCache[path.Value] = modState
				dbProviderContext = modState.Ctx
			}
		}

		preparationStartTime := time.Now()

		modState, mod, _, err := core.PrepareLocalModule(core.ModulePreparationArgs{
			Fpath:                     absEntryPath,
			ParsingCompilationContext: dbProviderContext,
			SingleFileParsingTimeout:  SINGLE_FILE_PARSING_TIMEOUT,

			ParentContext:         dbProviderContext,
			ParentContextRequired: true,
			DefaultLimits: []core.Limit{
				core.MustMakeNotAutoDepletingCountLimit(fs_ns.FS_READ_LIMIT_NAME, 10_000_000),
			},

			Out:                     io.Discard,
			DataExtractionMode:      true,
			ScriptContextFileSystem: constructionState.fls,
			PreinitFilesystem:       constructionState.fls,
		})

		if modState != nil {
			defer modState.Ctx.CancelGracefully()
		}

		if err != nil {
			pstate.lock.Lock()
			defer pstate.lock.Unlock()

			errorIndex = len(pstate.errors)
			pstate.errors = append(pstate.errors, err)
			pstate.operations = append(pstate.operations, operation)
			pstate.endpoints = append(pstate.endpoints, endpt)
			return
		}

		cacheKey := modState.EffectivePreparationParameters.PreparationCacheKey
		cacheKey.DataExtractionMode = false //Prevent core.PreparedLocalModule to refuse using the cache.

		operation.handlerModule = core.NewPreparationCacheEntry(cacheKey, core.PreparationCacheEntryUpdate{
			Time:            preparationStartTime,
			Module:          mod,
			StaticCheckData: modState.StaticCheckData,
			SymbolicData:    modState.SymbolicData.Data,
			//There should not be a symbolic check error because we handle the error returned by PrepareLocalModule.
		})

		bodyParams := utils.FilterSlice(modState.Manifest.Parameters.NonPositionalParameters(), func(p core.ModuleParameter) bool {
			return !strings.HasPrefix(p.Name(), "_")
		})

		if len(bodyParams) > 0 {
			if method == "GET" {
				pstate.lock.Lock()
				defer pstate.lock.Unlock()

				errorIndex = len(pstate.errors)
				pstate.errors = append(pstate.errors, fmt.Errorf("%w: module %q", ErrUnexpectedBodyParamsInGETHandler, absEntryPath))
				pstate.operations = append(pstate.operations, operation)
				pstate.endpoints = append(pstate.endpoints, endpt)
			} else if method == "OPTIONS" {
				pstate.lock.Lock()
				defer pstate.lock.Unlock()

				errorIndex = len(pstate.errors)
				pstate.errors = append(pstate.errors, fmt.Errorf("%w: module %q", ErrUnexpectedBodyParamsInOPTIONSHandler, absEntryPath))
				pstate.operations = append(pstate.operations, operation)
				pstate.endpoints = append(pstate.endpoints, endpt)
				return
			}

			var paramEntries []core.ObjectPatternEntry

			for _, param := range bodyParams {
				name := param.Name()
				paramEntries = append(paramEntries, core.ObjectPatternEntry{
					Name:       name,
					IsOptional: false,
					Pattern:    param.Pattern(),
				})
			}

			operation.jsonRequestBody = core.NewInexactObjectPattern(paramEntries)
		}
	}()

	return nil
}
