package http_ns

import (
	"errors"
	"fmt"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"slices"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/inoxlang/inox/internal/globals/html_ns"
	"github.com/inoxlang/inox/internal/globals/http_ns/spec"
	"github.com/inoxlang/inox/internal/inoxconsts"
	"github.com/inoxlang/inox/internal/mimeconsts"
	"github.com/inoxlang/inox/internal/mod"
	"github.com/inoxlang/inox/internal/utils"
)

const (
	INOX_FILE_EXTENSION = inoxconsts.INOXLANG_FILE_EXTENSION

	FS_ROUTING_LOG_SRC = "fs-routing"
)

var (
	//methods allowed in handler module filenames.
	FS_ROUTING_METHODS = spec.FS_ROUTING_METHODS
)

func addFilesystemRoutingHandler(server *HttpsServer, staticDir, dynamicDir core.Path, isMiddleware bool) error {
	if isMiddleware {
		return errors.New("filesystem routing handler cannot be used as a middleware")
	}

	fls := server.state.Ctx.GetFileSystem()

	var handleDynamic handlerFn
	if dynamicDir != "" {
		if _, err := fls.Stat(string(dynamicDir)); os.IsNotExist(err) {
			return fmt.Errorf("directory %q does not exist", dynamicDir)
		}
		handleDynamic = createHandleDynamic(server, dynamicDir)
	}

	if staticDir != "" {
		if _, err := fls.Stat(string(staticDir)); os.IsNotExist(err) {
			return fmt.Errorf("directory %q does not exist", staticDir)
		}
	}

	handler := func(req *HttpRequest, rw *HttpResponseWriter, handlerGlobalState *core.GlobalState) {

		if staticDir != "" {
			staticFilePath := staticDir.JoinAbsolute(req.Path, handlerGlobalState.Ctx.GetFileSystem())

			if staticFilePath.IsDirPath() {
				staticFilePath += "index.html"
			}

			fileExtension := filepath.Ext(string(staticFilePath))

			if fs_ns.Exists(handlerGlobalState.Ctx, staticFilePath) {

				//add CSP header if the content is HTML.
				if mimeconsts.IsMimeTypeForExtension(mimeconsts.HTML_CTYPE, fileExtension) {
					headerValue := core.String(server.defaultCSP.HeaderValue(CSPHeaderValueParams{}))
					rw.AddHeader(handlerGlobalState.Ctx, CSP_HEADER_NAME, headerValue)
				}

				err := serveFile(fileServingParams{
					ctx:            handlerGlobalState.Ctx,
					rw:             rw.DetachRespWriter(),
					r:              req.request,
					pth:            staticFilePath,
					fileCompressor: server.fileCompressor,
				})

				if err != nil {
					handlerGlobalState.Logger.Err(err).Send()
					rw.writeHeaders(http.StatusNotFound)
					return
				}
				return
			}
		}

		if handleDynamic != nil {
			handleDynamic(req, rw, handlerGlobalState)
		}

		if staticDir == "" && dynamicDir == "" {
			rw.DetachRespWriter().Write([]byte(NO_HANDLER_PLACEHOLDER_MESSAGE))
		}
	}

	server.lastHandlerFn = handler

	dynamicDirString := ""
	if dynamicDir != "" {
		dynamicDirString = dynamicDir.UnderlyingString()
	}

	api, err := spec.GetFSRoutingServerAPI(server.state.Ctx, dynamicDirString, spec.ServerApiResolutionConfig{})
	if err != nil {
		return err
	}

	server.api = api

	if dynamicDir != "" {
		// update the API each time the files are changed.
		server.onIdleFilesystem(idleFilesystemHandler{
			watchedPaths: []core.PathPattern{dynamicDir.ToPrefixPattern()},
			microtask: func(serverCtx *core.Context) {
				select {
				case <-serverCtx.Done():
					return
				default:
				}

				updatedAPI, err := spec.GetFSRoutingServerAPI(serverCtx, dynamicDirString, spec.ServerApiResolutionConfig{})

				if err != nil {
					serverCtx.Logger().Debug().Err(err).Send()
					return
				}

				select {
				case <-serverCtx.Done():
					return
				default:
				}

				server.apiLock.Lock()
				server.api = updatedAPI
				server.apiLock.Unlock()
			},
		})
	}

	// preparedModules := newPreparedModules(server.state.Ctx)
	// err = preparedModules.prepareFrom(api)
	// 	return err
	// }

	return nil
}

func createHandleDynamic(server *HttpsServer, routingDirPath core.Path) handlerFn {
	return func(req *HttpRequest, rw *HttpResponseWriter, handlerGlobalState *core.GlobalState) {
		path := req.Path
		method := req.Method.UnderlyingString()
		tx := handlerGlobalState.Ctx.GetTx()
		if tx == nil {
			panic(core.ErrUnreachable)
		}

		//check path
		if !path.IsAbsolute() {
			panic(core.ErrUnreachable)
		}

		if path.IsDirPath() && path != "/" {
			rw.writeHeaders(http.StatusNotFound)
			return
		}

		if slices.Contains(strings.Split(path.UnderlyingString(), "/"), "..") {
			rw.writeHeaders(http.StatusNotFound)
			return
		}

		// -----
		if strings.Contains(method, "/") {
			rw.writeHeaders(http.StatusNotFound)
			return
		}

		searchedMethod := method
		switch method {
		case "HEAD":
			searchedMethod = "GET"
		}

		server.apiLock.Lock()
		api := server.api
		server.apiLock.Unlock()

		endpt, err := api.GetEndpoint(string(path))
		if errors.Is(err, spec.ErrEndpointNotFound) {
			rw.writeHeaders(http.StatusNotFound)
			return
		}

		if err != nil {
			handlerGlobalState.Logger.Err(err).Send()
		}

		methodSpecificModule := true
		var module *core.Module

		if endpt.CatchAll() {
			methodSpecificModule = false
			module, _ = endpt.CatchAllHandler()
		} else {
			for _, operation := range endpt.Operations() {
				if operation.HttpMethod() == searchedMethod {
					module = utils.MustGet(operation.HandlerModule())
					break
				}
			}
		}

		if module == nil {
			rw.writeHeaders(http.StatusNotFound)
			return
		}
		modulePath := module.Name()
		handlerCtx := handlerGlobalState.Ctx

		//TODO: check the file is not writable

		preparationStart := time.Now()

		fsRoutingLogger := handlerGlobalState.Ctx.NewChildLoggerForInternalSource(FS_ROUTING_LOG_SRC)
		fsRoutingLogger = fsRoutingLogger.With().Str("handler", modulePath).Logger()
		moduleLogger := handlerGlobalState.Logger

		state, _, _, err := core.PrepareLocalModule(core.ModulePreparationArgs{
			Fpath:                 modulePath,
			CachedModule:          module,
			ParentContext:         handlerCtx,
			ParentContextRequired: true,
			DefaultLimits:         core.GetDefaultRequestHandlingLimits(),

			ParsingCompilationContext: handlerCtx,
			Out:                       handlerGlobalState.Out,
			Logger:                    moduleLogger,
			LogLevels:                 server.state.LogLevels,

			FullAccessToDatabases: false, //databases should be passed by parent state
			PreinitFilesystem:     handlerCtx.GetFileSystem(),
			GetArguments: func(manifest *core.Manifest) (*core.ModuleArgs, error) {
				args, errStatusCode, err := getHandlerModuleArguments(req, manifest, handlerCtx, methodSpecificModule)
				if err != nil {
					rw.writeHeaders(errStatusCode)
				}
				return args, err
			},
			BeforeContextCreation: func(m *core.Manifest) ([]core.Limit, error) {
				var defaultLimits map[string]core.Limit = maps.Clone(server.defaultLimits)

				//check the manifest's limits against the server's maximum limits
				//and remove present limits from defaultLimits.
				for _, limit := range m.Limits {
					maxLimit, ok := server.maxLimits[limit.Name]
					if ok && maxLimit.MoreRestrictiveThan(limit) {
						return nil, fmt.Errorf(
							"limit %q of handler module %q is higher than the maximum limit allowed, "+
								"note that you can configure the %s argument of the HTTP server",
							limit.Name, modulePath, HANDLING_DESC_MAX_LIMITS_PROPNAME)
					}

					delete(defaultLimits, limit.Name)
				}

				//add remaining defaultLimits.
				limits := slices.Clone(m.Limits)

				for _, limit := range defaultLimits {
					limits = append(limits, limit)
				}
				return limits, nil
			},
		})

		if err != nil {
			fsRoutingLogger.Err(err).Send()
			if !rw.IsStatusSent() {
				rw.writeHeaders(http.StatusInternalServerError)
			}
			if !handlerCtx.IsDoneSlowCheck() {
				tx.Rollback(handlerCtx)
			}
			return
		}

		fsRoutingLogger.Debug().Dur("preparation-time", time.Since(preparationStart)).Send()

		var debugger *core.Debugger

		if parentDebugger, _ := server.state.Debugger.Load().(*core.Debugger); parentDebugger != nil {
			debugger = parentDebugger.NewChild()
		}

		//run the handler module in the current goroutine.
		//The CPU time depletion of the handler is paused because the same corresponding depletion in the module's limiter is going to start.

		handlerCtx.PauseCPUTimeDepletion()

		result, _, _, _, err := mod.RunPreparedModule(mod.RunPreparedModuleArgs{
			State: state,

			ParentContext:             handlerCtx,
			ParsingCompilationContext: handlerCtx,
			IgnoreHighRiskScore:       true,
			Debugger:                  debugger,

			DoNotCancelWhenFinished: true,
		})

		handlerCtx.ResumeCPUTimeDepletion()

		if err != nil {
			handlerGlobalState.Logger.Err(err).Send()

			if handlerCtx.IsDoneSlowCheck() {
				if !rw.IsStatusSent() {
					rw.writeHeaders(http.StatusInternalServerError)
				}
			} else {
				if !rw.IsStatusSent() {
					rw.writeHeaders(http.StatusNotFound)
				}
			}

			tx.Rollback(handlerCtx)
			return
		}

		nonce := randomCSPNonce()

		//add nonce to <script> tags
		if node, ok := result.(*html_ns.HTMLNode); ok {
			node.AddNonceToScriptTagsNoEvent(nonce)
		}

		respondWithMappingResult(handlingArguments{
			value:        result,
			req:          req,
			rw:           rw,
			state:        handlerGlobalState,
			server:       server,
			logger:       handlerGlobalState.Logger,
			scriptsNonce: nonce,
			isMiddleware: false,
		})
	}
}

func getHandlerModuleArguments(req *HttpRequest, manifest *core.Manifest, handlerCtx *core.Context, methodSpecificModule bool) (
	_ *core.ModuleArgs,
	errStatusCode int,
	_ error,
) {

	if len(manifest.Parameters.PositionalParameters()) > 0 {
		return nil, http.StatusNotFound, errors.New("there should not be positional parameters")
	}

	handlerModuleParams, err := getHandlerModuleParameters(handlerCtx, manifest, methodSpecificModule)
	if err != nil {
		return nil, http.StatusNotFound, err
	}

	moduleArguments := map[string]core.Value{}
	method := core.Identifier(req.Method)

	if handlerModuleParams.methodPattern != nil {
		if !handlerModuleParams.methodPattern.Test(handlerCtx, method) {
			return nil, http.StatusBadRequest, errors.New("method is not accepted")
		}
		moduleArguments[spec.FS_ROUTING_METHOD_PARAM] = method
	}

	if handlerModuleParams.bodyReader {
		moduleArguments[spec.FS_ROUTING_BODY_PARAM] = req.Body
	} else if handlerModuleParams.jsonBodyPattern != nil {
		if !req.ContentType.MatchText(mimeconsts.JSON_CTYPE) {
			return nil, http.StatusBadRequest, errors.New("unsupported content type")
		}
		bytes, err := req.Body.ReadAll()
		if err != nil {
			return nil, http.StatusBadRequest, fmt.Errorf("failed to get arguments from body: %w", err)
		}

		v, err := core.ParseJSONRepresentation(handlerCtx, string(bytes.UnderlyingBytes()), handlerModuleParams.jsonBodyPattern)
		if err != nil {
			return nil, http.StatusBadRequest, fmt.Errorf("failed to get arguments from body: %w", err)
		}

		obj, ok := v.(*core.Object)
		if !ok {
			return nil, http.StatusBadRequest, errors.New("JSON body should be an object")
		}

		if !handlerModuleParams.jsonBodyPattern.Test(handlerCtx, obj) {
			return nil, http.StatusBadRequest, errors.New("request's body does not match module parameters")
		}

		handlerModuleParams.jsonBodyPattern.ForEachEntry(func(entry core.ObjectPatternEntry) error {
			moduleArguments[entry.Name] = obj.Prop(handlerCtx, entry.Name)
			return nil
		})
	} else { //body is not required by the handler
		if !methodSpecificModule && handlerModuleParams.methodPattern == nil && !req.IsGetOrHead() {
			return nil, http.StatusBadRequest, errors.New("only GET & HEAD requests are supported by the handler")
		}
	}
	return core.NewStructFromMap(moduleArguments), 0, nil
}

func getHandlerModuleParameters(ctx *core.Context, manifest *core.Manifest, methodSpecificModule bool) (handlerModuleParameters, error) {
	if len(manifest.Parameters.PositionalParameters()) > 0 {
		return handlerModuleParameters{}, errors.New("there should not be positional parameters")
	}

	var handlerModuleParams handlerModuleParameters
	var jsonBodyParams []core.ModuleParameter
	nonPositionalParams := manifest.Parameters.NonPositionalParameters()

	for _, param := range nonPositionalParams {
		paramName := param.Name()

		if paramName[0] == '_' {
			switch paramName {
			case spec.FS_ROUTING_METHOD_PARAM:
				handlerModuleParams.methodPattern = param.Pattern()
			case spec.FS_ROUTING_BODY_PARAM:
				if param.Pattern() != core.READER_PATTERN {
					return handlerModuleParameters{}, fmt.Errorf("parameter '%s' should have %%reader as pattern", paramName)
				}
				handlerModuleParams.bodyReader = true
				if jsonBodyParams != nil {
					return handlerModuleParameters{}, errors.New("parameter _body should not be present since some body parameters are specified")
				}
			default:
				return handlerModuleParameters{}, fmt.Errorf("unknown parameter name '%s'", paramName)
			}
			continue
		}

		if !methodSpecificModule {
			return handlerModuleParameters{}, fmt.Errorf("unexpected body parameter '%s': handler module is not method specific", paramName)
		}

		if handlerModuleParams.bodyReader {
			return handlerModuleParameters{}, errors.New("parameter _body should not be present since some body parameters are specified")
		}

		jsonBodyParams = append(jsonBodyParams, param)
	}

	if jsonBodyParams != nil {
		var entries []core.ObjectPatternEntry

		for _, param := range jsonBodyParams {
			entry := core.ObjectPatternEntry{
				Name:       param.Name(),
				Pattern:    param.Pattern(),
				IsOptional: param.Required(ctx),
			}
			entries = append(entries, entry)
		}

		handlerModuleParams.jsonBodyPattern = core.NewInexactObjectPattern(entries)
	}

	return handlerModuleParams, nil
}

type handlerModuleParameters struct {
	methodPattern   core.Pattern
	bodyReader      bool
	jsonBodyPattern *core.ObjectPattern //only for method-specific modules
}
