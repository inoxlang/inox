package http_ns

import (
	"errors"
	"fmt"
	"maps"
	"net/http"
	"strings"
	"time"

	"slices"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/inoxconsts"
	"github.com/inoxlang/inox/internal/mimeconsts"
	"github.com/inoxlang/inox/internal/mod"
)

const (
	INOX_FILE_EXTENSION     = inoxconsts.INOXLANG_FILE_EXTENSION
	FS_ROUTING_INDEX_MODULE = "index" + INOX_FILE_EXTENSION

	FS_ROUTING_BODY_PARAM   = "_body"
	FS_ROUTING_METHOD_PARAM = "_method"
)

var (
	//methods allowed in handler module filenames.
	FS_ROUTING_METHODS = []string{"GET", "OPTIONS", "POST", "PATCH", "PUT", "DELETE"}
)

func createHandleDynamic(server *HttpsServer, routingDirPath core.Path) handlerFn {
	return func(req *HttpRequest, rw *HttpResponseWriter, handlerGlobalState *core.GlobalState) {
		path := req.Path
		method := req.Method.UnderlyingString()

		//check path
		if !path.IsAbsolute() {
			panic(core.ErrUnreachable)
		}

		if path.IsDirPath() && path != "/" {
			rw.writeStatus(http.StatusNotFound)
			return
		}

		if slices.Contains(strings.Split(path.UnderlyingString(), "/"), "..") {
			rw.writeStatus(http.StatusNotFound)
			return
		}

		// -----
		if strings.Contains(method, "/") {
			rw.writeStatus(http.StatusNotFound)
			return
		}

		searchedMethod := method
		switch method {
		case "HEAD":
			searchedMethod = "GET"
		}

		endpt, err := server.api.GetEndpoint(string(path))
		if errors.Is(err, ErrEndpointNotFound) {
			rw.writeStatus(http.StatusNotFound)
			return
		}

		if err != nil {
			handlerGlobalState.Logger.Err(err).Send()
		}

		methodSpecificModule := true
		var module *core.Module

		if endpt.catchAll {
			methodSpecificModule = false
			module = endpt.catchAllHandler
		} else {
			for _, operation := range endpt.operations {
				if operation.httpMethod == searchedMethod {
					module = operation.handlerModule
					break
				}
			}
		}

		if module == nil {
			rw.writeStatus(http.StatusNotFound)
			return
		}
		modulePath := module.Name()
		handlerCtx := handlerGlobalState.Ctx

		//TODO: check the file is not writable

		preparationStart := time.Now()
		logger := handlerGlobalState.Logger.With().Str("handler-module", modulePath).Logger()

		state, _, _, err := core.PrepareLocalScript(core.ScriptPreparationArgs{
			Fpath:                       modulePath,
			CachedModule:                module,
			ParentContext:               handlerCtx,
			ParentContextRequired:       true,
			DoNotAddDefaultScriptLimits: true,

			ParsingCompilationContext: handlerCtx,
			Out:                       handlerGlobalState.Out,
			LogOut:                    logger,

			FullAccessToDatabases: false, //databases should be passed by parent state
			PreinitFilesystem:     handlerCtx.GetFileSystem(),
			GetArguments: func(manifest *core.Manifest) (*core.Struct, error) {
				args, errStatusCode, err := getHandlerModuleArguments(req, manifest, handlerCtx, methodSpecificModule)
				if err != nil {
					rw.writeStatus(core.Int(errStatusCode))
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
			logger.Err(err).Send()
			if !rw.isStatusSet() {
				rw.writeStatus(http.StatusInternalServerError)
			}
			if !handlerCtx.IsDoneSlowCheck() {
				tx := handlerCtx.GetTx()
				if tx != nil {
					tx.Rollback(handlerCtx)
				}
			}
			return
		}

		logger.Debug().Dur("preparation-time", time.Since(preparationStart)).Send()

		var debugger *core.Debugger

		if parentDebugger, _ := server.state.Debugger.Load().(*core.Debugger); parentDebugger != nil {
			debugger = parentDebugger.NewChild()
		}

		//run the handler module in the current goroutine.
		//CPU time decrementation is paused because the module will start decrementing its CPU time.

		handlerCtx.PauseCPUTimeDecrementation()

		result, _, _, _, err := mod.RunPreparedScript(mod.RunPreparedScriptArgs{
			State: state,

			ParentContext:             handlerCtx,
			ParsingCompilationContext: handlerCtx,
			IgnoreHighRiskScore:       true,
			Debugger:                  debugger,
		})

		handlerCtx.ResumeCPUTimeDecrementation()

		if err != nil {
			handlerGlobalState.Logger.Err(err).Send()

			if !handlerCtx.IsDoneSlowCheck() {
				if !rw.isStatusSet() {
					rw.writeStatus(http.StatusNotFound)
				}

				tx := handlerCtx.GetTx()
				if tx != nil {
					tx.Rollback(handlerCtx)
				}
			} else if !rw.isStatusSet() { //context is done
				rw.writeStatus(http.StatusInternalServerError)
			}
			return
		}

		respondWithMappingResult(handlingArguments{
			value:        result,
			req:          req,
			rw:           rw,
			state:        handlerGlobalState,
			server:       server,
			logger:       handlerGlobalState.Logger,
			isMiddleware: false,
		})
	}
}

func getHandlerModuleArguments(req *HttpRequest, manifest *core.Manifest, handlerCtx *core.Context, methodSpecificModule bool) (
	_ *core.Struct,
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
		moduleArguments[FS_ROUTING_METHOD_PARAM] = method
	}

	if handlerModuleParams.bodyReader {
		moduleArguments[FS_ROUTING_BODY_PARAM] = req.Body
	} else if handlerModuleParams.jsonBodyPattern != nil {
		if !req.ContentType.MatchText(mimeconsts.JSON_CTYPE) {
			return nil, http.StatusBadRequest, errors.New("unsupported content type")
		}
		bytes, err := req.Body.ReadAll()
		if err != nil {
			return nil, http.StatusBadRequest, fmt.Errorf("failed to get arguments from body: %w", err)
		}

		v, err := core.ParseJSONRepresentation(handlerCtx, string(bytes.Bytes), handlerModuleParams.jsonBodyPattern)
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

		handlerModuleParams.jsonBodyPattern.ForEachEntry(func(propName string, propPattern core.Pattern, isOptional bool) error {
			moduleArguments[propName] = obj.Prop(handlerCtx, propName)
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
			case FS_ROUTING_METHOD_PARAM:
				handlerModuleParams.methodPattern = param.Pattern()
			case FS_ROUTING_BODY_PARAM:
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
		entries := map[string]core.Pattern{}
		optionalEntries := map[string]struct{}{}

		for _, param := range jsonBodyParams {
			entries[param.Name()] = param.Pattern()
			if !param.Required(ctx) {
				optionalEntries[param.Name()] = struct{}{}
			}
		}

		handlerModuleParams.jsonBodyPattern = core.NewInexactObjectPatternWithOptionalProps(entries, optionalEntries)
	}

	return handlerModuleParams, nil
}

type handlerModuleParameters struct {
	methodPattern   core.Pattern
	bodyReader      bool
	jsonBodyPattern *core.ObjectPattern //only for method-specific modules
}
