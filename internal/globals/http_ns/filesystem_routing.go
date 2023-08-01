package http_ns

import (
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"

	core "github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/inox_ns"
	"golang.org/x/exp/slices"
)

func createHandleDynamic(server *HttpServer, routingDirPath core.Path) handlerFn {
	return func(req *HttpRequest, rw *HttpResponseWriter, handlerGlobalState *core.GlobalState) {
		fls := server.state.Ctx.GetFileSystem()
		path := req.Path
		method := req.Method.UnderlyingString()

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

		if strings.Contains(method, "/") {
			rw.writeStatus(http.StatusNotFound)
			return
		}

		//test different paths for the module

		pathDir, pathBasename := filepath.Split(string(path))
		modulePath := fls.Join(string(routingDirPath), pathDir, "/"+string(req.Method)+"-"+pathBasename+".ix")
		methodSpecificModule := true

		_, err := fls.Stat(modulePath)
		if err != nil {
			modulePath = fls.Join(string(routingDirPath), string(path)+".ix")
			methodSpecificModule = false
		}

		_, err = fls.Stat(modulePath)
		if err != nil {
			modulePath = fls.Join(string(routingDirPath), string(path), "index.ix")
		}

		_, err = fls.Stat(modulePath)
		if err != nil {
			rw.writeStatus(http.StatusNotFound)
			return
		}

		handlerCtx := handlerGlobalState.Ctx

		//TODO: check the file is not writable

		state, _, _, err := inox_ns.PrepareLocalScript(inox_ns.ScriptPreparationArgs{
			Fpath:                 modulePath,
			ParentContext:         handlerCtx,
			ParentContextRequired: true,

			ParsingCompilationContext: handlerCtx,
			Out:                       handlerGlobalState.Out,
			LogOut:                    handlerGlobalState.Logger,

			FullAccessToDatabases: false, //databases should be passed by parent state
			PreinitFilesystem:     handlerCtx.GetFileSystem(),

			GetArguments: func(manifest *core.Manifest) (*core.Struct, error) {
				args, errStatusCode, err := getHandlerModuleArguments(req, manifest, handlerCtx, methodSpecificModule)
				if err != nil {
					rw.writeStatus(core.Int(errStatusCode))
				}
				return args, err
			},
		})

		if err != nil {
			handlerGlobalState.Logger.Err(err).Str("handler-module", modulePath).Send()
			return
		}

		result, _, _, _, err := inox_ns.RunPreparedScript(inox_ns.RunPreparedScriptArgs{
			State: state,

			ParentContext:             handlerCtx,
			ParsingCompilationContext: handlerCtx,
			IgnoreHighRiskScore:       true,
		})

		if err != nil {
			handlerGlobalState.Logger.Err(err).Send()
			rw.writeStatus(http.StatusNotFound)
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
		moduleArguments["_method"] = method
	}

	if handlerModuleParams.bodyReader {
		moduleArguments["_body"] = req.Body
	} else if handlerModuleParams.jsonBodyPattern != nil {
		if !req.ContentType.MatchText(core.JSON_CTYPE) {
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
			case "_method":
				handlerModuleParams.methodPattern = param.Pattern()
			case "_body":
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
