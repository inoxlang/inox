package http_ns

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"slices"
	"strings"

	core "github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/mod"
	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/utils"
)

var (
	ErrUnexpectedBodyParamsInGETHandler     = errors.New("unexpected request body parmameters in GET handler")
	ErrUnexpectedBodyParamsInOPTIONSHandler = errors.New("unexpected request body parmameters in OPTIONS handler")
)

func getServerAPI(ctx *core.Context, server *HttpServer) {

}

func getFilesystemRoutingServerAPI(ctx *core.Context, dir string) (*API, error) {
	api := &API{
		endpoints: map[string]*ApiEndpoint{},
	}

	preparedModuleCache := map[string]*core.GlobalState{}
	defer func() {
		for _, state := range preparedModuleCache {
			state.Ctx.CancelGracefully()
		}
	}()

	err := addFilesysteDirEndpoints(ctx, api, dir, "/", preparedModuleCache)
	if err != nil {
		return nil, err
	}

	return api, nil
}

// addFilesysteDirEndpoints recursively add the endpoints provided by dir and its subdirectories.
func addFilesysteDirEndpoints(ctx *core.Context, api *API, dir, urlDirPath string, preparedModuleCache map[string]*core.GlobalState) error {
	fls := ctx.GetFileSystem()
	entries, err := fls.ReadDir(dir)

	dir = core.AppendTrailingSlashIfNotPresent(dir)
	urlDirPath = core.AppendTrailingSlashIfNotPresent(urlDirPath)

	if err != nil {
		return err
	}

	urlDirPathNoTrailingSlash := strings.TrimSuffix(urlDirPath, "/")
	if urlDirPath == "/" {
		urlDirPathNoTrailingSlash = "/"
	}

	_ = urlDirPathNoTrailingSlash
	for _, entry := range entries {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		entryName := entry.Name()
		absEntryPath := filepath.Join(dir, entryName)

		if entry.IsDir() {
			subDir := absEntryPath + "/"
			urlSubDir := filepath.Join(urlDirPath, entryName) + "/"

			err := addFilesysteDirEndpoints(ctx, api, subDir, urlSubDir, preparedModuleCache)
			if err != nil {
				return err
			}
			continue
		}

		//ignore non-Inox files
		if !strings.HasSuffix(entryName, INOX_FILE_EXTENSION) {
			continue
		}

		entryNameNoExt := strings.TrimSuffix(entryName, INOX_FILE_EXTENSION)

		var endpointPath string
		var method string //if empty the handler module supports several methods
		ignoreIfNotModule := false

		if slices.Contains(FS_ROUTING_METHODS, entryNameNoExt) { //GET.ix, POST.ix, ...
			//add operation
			method = entryNameNoExt
			endpointPath = urlDirPathNoTrailingSlash
		} else {
			beforeName, name, ok := strings.Cut(entryNameNoExt, "-")

			if ok && slices.Contains(FS_ROUTING_METHODS, beforeName) { //POST-... , GET-...
				method = beforeName
				endpointPath = filepath.Join(urlDirPath, name)
			} else if entryName == FS_ROUTING_INDEX_MODULE { //index.ix
				endpointPath = urlDirPathNoTrailingSlash
			} else { //example: about.ix
				endpointPath = filepath.Join(urlDirPath, entryNameNoExt)
				ignoreIfNotModule = true
			}
		}

		chunk, err := core.ParseFileChunk(absEntryPath, fls)
		if err != nil {
			return fmt.Errorf("failed to parse %q: %w", absEntryPath, err)
		}

		if chunk.Node.Manifest == nil { //not a module
			if ignoreIfNotModule {
				continue
			}
			return fmt.Errorf("%q is not a module", absEntryPath)
		}

		//add operation
		endpt := api.endpoints[endpointPath]
		if endpt == nil {
			endpt = &ApiEndpoint{}
			api.endpoints[endpointPath] = endpt
		}

		//check the same operation is not already defined
		for _, op := range endpt.operations {
			if op.httpMethod == method || method == "" {
				if op.handlerModule != nil {
					return fmt.Errorf(
						"operation %s %q is already implemented by the module %q; unexpected module %q",
						op.httpMethod, endpointPath, op.handlerModule.Name(), absEntryPath)
				}
				return fmt.Errorf(
					"operation %s %q is already implemented; unexpected module %q",
					op.httpMethod, endpointPath, absEntryPath)
			}
		}

		endpt.operations = append(endpt.operations, ApiOperation{
			httpMethod: method,
		})
		operation := &endpt.operations[len(endpt.operations)-1]

		manifestObj := chunk.Node.Manifest.Object.(*parse.ObjectLiteral)
		dbSection, _ := manifestObj.PropValue(core.MANIFEST_DATABASES_SECTION_NAME)

		var parentCtx *core.Context = ctx

		if path, ok := dbSection.(*parse.AbsolutePathLiteral); ok {
			if cache, ok := preparedModuleCache[path.Value]; ok {
				parentCtx = cache.Ctx
			} else {
				state, _, _, err := mod.PrepareLocalScript(mod.ScriptPreparationArgs{
					Fpath:                     path.Value,
					ParsingCompilationContext: ctx,

					ParentContext:         ctx,
					ParentContextRequired: true,

					Out:                     io.Discard,
					DataExtractionMode:      true,
					ScriptContextFileSystem: fls,
					PreinitFilesystem:       fls,
				})

				if err != nil {
					return err
				}

				preparedModuleCache[path.Value] = state
			}
		}

		state, mod, _, err := mod.PrepareLocalScript(mod.ScriptPreparationArgs{
			Fpath:                     absEntryPath,
			ParsingCompilationContext: parentCtx,

			ParentContext:         parentCtx,
			ParentContextRequired: true,

			Out:                     io.Discard,
			DataExtractionMode:      true,
			ScriptContextFileSystem: fls,
			PreinitFilesystem:       fls,
		})

		if state != nil {
			defer state.Ctx.CancelGracefully()
		}

		if err != nil {
			return err
		}

		operation.handlerModule = mod

		bodyParams := utils.FilterSlice(state.Manifest.Parameters.NonPositionalParameters(), func(p core.ModuleParameter) bool {
			return !strings.HasPrefix(p.Name(), "_")
		})

		if len(bodyParams) > 0 {
			if method == "GET" {
				return fmt.Errorf("%w: module %q", ErrUnexpectedBodyParamsInGETHandler, absEntryPath)
			} else if method == "OPTIONS" {
				return fmt.Errorf("%w: module %q", ErrUnexpectedBodyParamsInOPTIONSHandler, absEntryPath)
			}

			paramPatterns := make(map[string]core.Pattern)
			optionalParams := make(map[string]struct{})

			for _, param := range bodyParams {
				name := param.Name()
				if !param.Required(ctx) {
					optionalParams[name] = struct{}{}
				}
				paramPatterns[name] = param.Pattern()
			}

			operation.jsonRequestBody = core.NewInexactObjectPatternWithOptionalProps(paramPatterns, optionalParams)
		}
	}

	return nil
}
