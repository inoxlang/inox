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
	IgnoreModulesWithErrors bool
}

func GetFSRoutingServerAPI(ctx *core.Context, dir string, config ServerApiResolutionConfig) (*API, error) {
	preparedModuleCache := map[string]*core.GlobalState{}
	defer func() {
		for _, state := range preparedModuleCache {
			state.Ctx.CancelGracefully()
		}
	}()

	endpoints := map[string]*ApiEndpoint{}
	pState := &parallelState{endpointLocks: map[*ApiEndpoint]*sync.Mutex{}}

	if dir != "" {
		err := addFilesysteDirEndpoints(ctx, config, endpoints, dir, "/", preparedModuleCache, pState)
		if err != nil {
			return nil, err
		}
	}

	return NewAPI(endpoints)
}

// addFilesysteDirEndpoints recursively add the endpoints provided by dir and its subdirectories.
func addFilesysteDirEndpoints(
	ctx *core.Context,
	config ServerApiResolutionConfig,
	endpoints map[string]*ApiEndpoint,
	dir,
	urlDirPath string,
	preparedModuleCache map[string]*core.GlobalState,
	pState *parallelState,
) error {
	fls := ctx.GetFileSystem()
	entries, err := fls.ReadDir(dir)

	//Normalize the directory and the URL directory.

	dir = core.AppendTrailingSlashIfNotPresent(dir)
	urlDirPath = core.AppendTrailingSlashIfNotPresent(urlDirPath)
	dirBasename := filepath.Base(dir)

	if err != nil {
		return err
	}

	urlDirPathNoTrailingSlash := strings.TrimSuffix(urlDirPath, "/")
	if urlDirPath == "/" {
		urlDirPathNoTrailingSlash = "/"
	}

	parentState, _ := ctx.GetState()

	wg := new(sync.WaitGroup) //This wait group is waited for in 2 different places.

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

			err := addFilesysteDirEndpoints(ctx, config, endpoints, subDir, urlSubDir, preparedModuleCache, pState)
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
		chunk, err := core.ParseFileChunk(absEntryPath, fls, parse.ParserOptions{
			Timeout: SINGLE_FILE_PARSING_TIMEOUT,
		})

		if err != nil {
			if config.IgnoreModulesWithErrors {
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

		//Add endpoint.
		endpt := endpoints[endpointPath]
		if endpt == nil && endpointPath == "/" {
			endpt = &ApiEndpoint{
				path: "/",
			}
			endpoints[endpointPath] = endpt
		} else if endpt == nil {
			//Add endpoint into the API.
			endpt = &ApiEndpoint{
				path: endpointPath,
			}
			endpoints[endpointPath] = endpt
			if endpointPath == "" || endpointPath[0] != '/' {
				return fmt.Errorf("invalid endpoint path %q", endpointPath)
			}
		}

		//We need to lock the endpoint because the same endpoint can be mutated by a goroutine created by the caller.
		pState.lock.Lock()
		endpointLock, ok := pState.endpointLocks[endpt]
		if !ok {
			endpointLock = new(sync.Mutex)
			pState.endpointLocks[endpt] = endpointLock
		}
		pState.lock.Unlock()

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
		go addHandlerModule(
			ctx.BoundChild(), parentState,

			//handler
			method, fls, absEntryPath, chunk,

			preparedModuleCache, config,

			//parallelization
			wg, endpointLock, pState,

			//API components
			endpt, operation,
		)
	}

	wg.Wait() //TODO: prevent blocking + add a timeout (kill context)

	//Handle errors
	for i, err := range pState.errors {
		if config.IgnoreModulesWithErrors {
			endpoint := pState.endpoints[i]
			operation := pState.operations[i]

			if endpoint == nil || operation == nil {
				continue
			}

			if len(endpoint.operations) == 1 {
				delete(endpoints, endpoint.path)
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
	for _, endpt := range endpoints {
		pState.lock.Lock()
		endpointLock := pState.endpointLocks[endpt]
		pState.lock.Unlock()

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

type parallelState struct {
	errors     []error
	operations []*ApiOperation //same length as .errors
	endpoints  []*ApiEndpoint  //same length as .errors

	endpointLocks map[*ApiEndpoint]*sync.Mutex

	lock sync.Mutex
}

func addHandlerModule(
	ctx *core.Context,
	parentState *core.GlobalState,

	method string,
	fls afs.Filesystem,
	absEntryPath string,
	chunk *parse.ParsedChunkSource,

	preparedModuleCache map[string]*core.GlobalState,
	config ServerApiResolutionConfig,

	wg *sync.WaitGroup,
	endpointLock *sync.Mutex,
	pState *parallelState,

	endpoint *ApiEndpoint,
	operation *ApiOperation,
) {

	defer wg.Done()
	defer endpointLock.Unlock()

	errorIndex := -1

	defer func() {
		e := recover()
		if e != nil {
			err := utils.ConvertPanicValueToError(e)
			err = fmt.Errorf("%w: %s", err, debug.Stack())

			if errorIndex >= 0 {
				pState.errors[errorIndex] = err
			} else {
				pState.lock.Lock()
				defer pState.lock.Unlock()
				pState.errors = append(pState.errors, err)
				pState.operations = append(pState.operations, nil)
				pState.endpoints = append(pState.endpoints, nil)
			}
		}
	}()

	defer ctx.CancelGracefully()

	manifestObj := chunk.Node.Manifest.Object.(*parse.ObjectLiteral)
	dbSection, _ := manifestObj.PropValue(core.MANIFEST_DATABASES_SECTION_NAME)

	var parentCtx *core.Context = ctx

	//If the databases are defined in another module we retrieve this module.
	if path, ok := dbSection.(*parse.AbsolutePathLiteral); ok {
		if cache, ok := preparedModuleCache[path.Value]; ok {
			parentCtx = cache.Ctx

			//if false there is nothing to do as the parentCtx is already set to ctx.
		} else if parentState.Module.Name() != path.Value {

			state, _, _, err := core.PrepareLocalModule(core.ModulePreparationArgs{
				Fpath:                     path.Value,
				ParsingCompilationContext: ctx,
				SingleFileParsingTimeout:  SINGLE_FILE_PARSING_TIMEOUT,

				ParentContext:         ctx,
				ParentContextRequired: true,
				DefaultLimits: []core.Limit{
					core.MustMakeNotAutoDepletingCountLimit(fs_ns.FS_READ_LIMIT_NAME, 10_000_000),
				},

				Out:                     io.Discard,
				DataExtractionMode:      true,
				ScriptContextFileSystem: fls,
				PreinitFilesystem:       fls,
			})

			if err != nil {
				pState.lock.Lock()
				defer pState.lock.Unlock()

				errorIndex = len(pState.errors)
				pState.errors = append(pState.errors, err)
				pState.operations = append(pState.operations, operation)
				pState.endpoints = append(pState.endpoints, endpoint)
				return
			}

			preparedModuleCache[path.Value] = state
			parentCtx = state.Ctx
		}
	}

	preparationStartTime := time.Now()

	state, mod, _, err := core.PrepareLocalModule(core.ModulePreparationArgs{
		Fpath:                     absEntryPath,
		ParsingCompilationContext: parentCtx,
		SingleFileParsingTimeout:  SINGLE_FILE_PARSING_TIMEOUT,

		ParentContext:         parentCtx,
		ParentContextRequired: true,
		DefaultLimits: []core.Limit{
			core.MustMakeNotAutoDepletingCountLimit(fs_ns.FS_READ_LIMIT_NAME, 10_000_000),
		},

		Out:                     io.Discard,
		DataExtractionMode:      true,
		ScriptContextFileSystem: fls,
		PreinitFilesystem:       fls,
	})

	if state != nil {
		defer state.Ctx.CancelGracefully()
	}

	if err != nil {
		pState.lock.Lock()
		defer pState.lock.Unlock()

		errorIndex = len(pState.errors)
		pState.errors = append(pState.errors, err)
		pState.operations = append(pState.operations, operation)
		pState.endpoints = append(pState.endpoints, endpoint)
		return
	}

	cacheKey := state.EffectivePreparationParameters.PreparationCacheKey

	operation.handlerModule = core.NewPreparationCacheEntry(cacheKey, core.PreparationCacheEntryUpdate{
		Time:            preparationStartTime,
		Module:          mod,
		StaticCheckData: state.StaticCheckData,
		SymbolicData:    state.SymbolicData.Data,
		//There should not be a symbolic check error because we handle the error returned by PrepareLocalModule.
	})

	bodyParams := utils.FilterSlice(state.Manifest.Parameters.NonPositionalParameters(), func(p core.ModuleParameter) bool {
		return !strings.HasPrefix(p.Name(), "_")
	})

	if len(bodyParams) > 0 {
		if method == "GET" {
			pState.lock.Lock()
			defer pState.lock.Unlock()

			errorIndex = len(pState.errors)
			pState.errors = append(pState.errors, fmt.Errorf("%w: module %q", ErrUnexpectedBodyParamsInGETHandler, absEntryPath))
			pState.operations = append(pState.operations, operation)
			pState.endpoints = append(pState.endpoints, endpoint)
		} else if method == "OPTIONS" {
			pState.lock.Lock()
			defer pState.lock.Unlock()

			errorIndex = len(pState.errors)
			pState.errors = append(pState.errors, fmt.Errorf("%w: module %q", ErrUnexpectedBodyParamsInOPTIONSHandler, absEntryPath))
			pState.operations = append(pState.operations, operation)
			pState.endpoints = append(pState.endpoints, endpoint)
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
}
