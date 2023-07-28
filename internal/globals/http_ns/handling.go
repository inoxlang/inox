package http_ns

import (
	"fmt"
	"html"
	"net/http"
	"strings"

	core "github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/inoxlang/inox/internal/globals/inox_ns"
	jsoniter "github.com/json-iterator/go"
	"github.com/rs/zerolog"
	"golang.org/x/exp/slices"
)

var (
	DEFAULT_CSP, _ = NewCSPWithDirectives(nil)

	HANDLER_DISABLED_ARGS = []bool{true, true}
)

func isValidHandlerValue(val core.Value) bool {
	switch val.(type) {
	case *core.InoxFunction, *core.GoFunction, *core.Mapping, core.Path, *core.Object:
		return true
	}
	return false
}

// a handlerFn is a middleware or the final handler
type handlerFn func(*HttpRequest, *HttpResponseWriter, *core.GlobalState)

func createHandlerFunction(handlerValue core.Value, isMiddleware bool, server *HttpServer) (handler handlerFn) {

	//set value for handler based on provided arguments
	switch userHandler := handlerValue.(type) {
	case *core.InoxFunction:
		handler = func(req *HttpRequest, rw *HttpResponseWriter, handlerGlobalState *core.GlobalState) {
			//call the Inox handler
			args := []core.Value{core.ValOf(rw), core.ValOf(req)}
			_, err := userHandler.Call(handlerGlobalState, nil, args, HANDLER_DISABLED_ARGS)

			if err != nil {
				handlerGlobalState.Logger.Print(err)
			}
		}
	case *core.GoFunction:
		handler = func(req *HttpRequest, rw *HttpResponseWriter, handlerGlobalState *core.GlobalState) {
			//call the Golang handler
			args := []any{rw, req}

			_, err := userHandler.Call(args, handlerGlobalState, nil, false, false)

			if err != nil {
				handlerGlobalState.Logger.Print(err)
			}
		}
	case core.Path:
		//filesystem routing

		routingDirPath := userHandler

		if !routingDirPath.IsAbsolute() || !routingDirPath.IsDirPath() {
			panic(fmt.Errorf("path of routing directory should be an absolute directory path"))
		}
		handler = createHandleDynamic(server, routingDirPath)
	case *core.Object:
		var staticDir core.Path
		var dynamicDir core.Path
		var handleDynamic handlerFn

		propertyNames := userHandler.PropertyNames(server.state.Ctx)
		if slices.Contains(propertyNames, "static") {
			staticDir = userHandler.Prop(server.state.Ctx, "static").(core.Path)
		}
		if slices.Contains(propertyNames, "dynamic") {
			dynamicDir = userHandler.Prop(server.state.Ctx, "dynamic").(core.Path)
			handleDynamic = createHandleDynamic(server, dynamicDir)
		}

		handler = func(req *HttpRequest, rw *HttpResponseWriter, handlerGlobalState *core.GlobalState) {

			if staticDir != "" {
				staticResourcePath := staticDir.JoinAbsolute(req.Path, handlerGlobalState.Ctx.GetFileSystem())

				if staticResourcePath.IsDirPath() {
					staticResourcePath += "index.html"
				}

				if fs_ns.Exists(handlerGlobalState.Ctx, staticResourcePath) {
					err := serveFile(handlerGlobalState.Ctx, rw, req, staticResourcePath)
					if err != nil {
						handlerGlobalState.Logger.Err(err).Send()
						rw.writeStatus(http.StatusNotFound)
						return
					}
					return
				}
			}

			if handleDynamic != nil {
				handleDynamic(req, rw, handlerGlobalState)
			}

			if staticDir == "" && dynamicDir == "" {
				rw.rw.Write([]byte(NO_HANDLER_PLACEHOLDER_MESSAGE))
			}
		}
	case *core.Mapping:
		routing := userHandler
		//if a routing Mapping is provided we compute a value by passing the request's path to the Mapping.
		handler = func(req *HttpRequest, rw *HttpResponseWriter, handlerGlobalState *core.GlobalState) {
			path := req.Path

			value := routing.Compute(handlerGlobalState.Ctx, path)
			if value == nil {
				handlerGlobalState.Logger.Print("routing mapping returned Go nil")
				rw.writeStatus(http.StatusNotFound)
				return
			}

			respondWithMappingResult(handlingArguments{value, req, rw, handlerGlobalState, server, handlerGlobalState.Logger, isMiddleware})
		}
	default:
		panic(core.ErrUnreachable)

	}

	return handler
}

func createHandleDynamic(server *HttpServer, routingDirPath core.Path) handlerFn {
	return func(req *HttpRequest, rw *HttpResponseWriter, handlerGlobalState *core.GlobalState) {
		fls := server.state.Ctx.GetFileSystem()
		path := req.Path
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

		modulePath := fls.Join(string(routingDirPath), string(path)+".ix")

		_, err := fls.Stat(modulePath)
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

		result, _, _, _, err := inox_ns.RunLocalScript(inox_ns.RunScriptArgs{
			Fpath:                     modulePath,
			ParentContext:             handlerCtx,
			ParentContextRequired:     true,
			ParsingCompilationContext: handlerCtx,
			Out:                       handlerGlobalState.Out,
			LogOut:                    handlerGlobalState.Logger,

			FullAccessToDatabases: false, //databases should be passed by parent state
			IgnoreHighRiskScore:   true,
			PreinitFilesystem:     handlerCtx.GetFileSystem(),
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

type handlingArguments struct {
	value        core.Value
	req          *HttpRequest
	rw           *HttpResponseWriter
	state        *core.GlobalState
	server       *HttpServer
	logger       zerolog.Logger
	isMiddleware bool
}

func respondWithMappingResult(h handlingArguments) {
	//TODO: log errors returned by response writer's methods

	value := h.value
	req := h.req
	rw := h.rw
	state := h.state
	logger := h.logger
	server := h.server
	renderingConfig := core.RenderingInput{Mime: core.HTML_CTYPE}

	switch v := value.(type) {
	case *core.InoxFunction: // if inox handler we call it and return
		args := []core.Value{core.ValOf(rw), core.ValOf(req)}
		_, err := v.Call(state, nil, args, HANDLER_DISABLED_ARGS)

		if err != nil {
			logger.Print("error when calling returned inox function:", err)
		}
		return
	case core.Identifier:
		switch v {
		case "notfound":
			rw.writeStatus(http.StatusNotFound)
			return
		case "continue":
			if h.isMiddleware {
				return
			}
			rw.writeStatus(http.StatusNotFound)
			return
		default:
			logger.Print("unknwon identifier " + string(v))
			rw.writeStatus(http.StatusNotFound)
			return
		}
	}

	//if JSON/IXON is accepted we serialize if possible.
	switch {
	case req.AcceptAny():
		break
	case req.ParsedAcceptHeader.Match(core.IXON_CTYPE):
		config := &core.ReprConfig{}

		serializable, ok := value.(core.Serializable)
		if !ok {
			rw.writeStatus(http.StatusNotAcceptable)
			return
		}

		rw.WriteContentType(core.IXON_CTYPE)
		serializable.WriteRepresentation(state.Ctx, rw.BodyWriter(), config, 0)
		return
	case req.ParsedAcceptHeader.Match(core.JSON_CTYPE):
		config := core.JSONSerializationConfig{
			ReprConfig: &core.ReprConfig{},
		}

		serializable, ok := value.(core.Serializable)
		if !ok {
			rw.writeStatus(http.StatusNotAcceptable)
			return
		}

		rw.WriteContentType(core.JSON_CTYPE)
		stream := jsoniter.NewStream(jsoniter.ConfigCompatibleWithStandardLibrary, rw.BodyWriter(), 0)
		serializable.WriteJSONRepresentation(state.Ctx, stream, config, 0)
		stream.Flush()
		return
	}

	switch req.Method {
	case "GET", "HEAD", "OPTIONS":
		break
	case "POST", "PATCH":
		switch {
		case req.ContentType.MatchText(core.APP_OCTET_STREAM_CTYPE):
			getData := func() ([]byte, bool) {
				b, err := req.Body.ReadAllBytes()

				if err != nil {
					logger.Print("failed to read request's body", err)
					rw.writeStatus(http.StatusInternalServerError)
					return nil, false
				}

				return b, true
			}

			if sink, ok := value.(core.StreamSink); ok {
				stream, ok := sink.WritableStream(state.Ctx, nil).(*core.WritableByteStream)
				if !ok {
					rw.writeStatus(http.StatusBadRequest)
					return
				}

				b, ok := getData()
				if !ok {
					return
				}

				if err := stream.WriteBytes(state.Ctx, b); err != nil {
					logger.Print("failed to write body to stream", err)
					rw.writeStatus(http.StatusInternalServerError)
					return
				}
			} else if v, ok := value.(core.Writable); ok {
				b, ok := getData()
				if !ok {
					return
				}

				if _, err := v.Writer().Write(b); err != nil {
					logger.Print("failed to write body to writable", err)
					rw.writeStatus(http.StatusInternalServerError)
				}

			} else {
				rw.writeStatus(http.StatusBadRequest)
				return
			}

			return
		}
	default:
		rw.writeStatus(http.StatusMethodNotAllowed)
		return
	}

	// rendering | event stream
loop:
	for {
		switch v := value.(type) {
		case core.NilT, nil:
			logger.Print("nil result")
			rw.writeStatus(http.StatusNotFound)
			return

		case core.StringLike:
			if !req.ParsedAcceptHeader.Match(core.PLAIN_TEXT_CTYPE) {
				rw.writeStatus(http.StatusNotAcceptable)
				return
			}

			//TODO: replace non printable characters
			escaped := html.EscapeString(v.GetOrBuildString())

			rw.WritePlainText(h.state.Ctx, &core.ByteSlice{Bytes: []byte(escaped)})
		case *core.ByteSlice:
			contentType := string(v.ContentType())
			if !req.ParsedAcceptHeader.Match(contentType) {
				rw.writeStatus(http.StatusNotAcceptable)
				return
			}

			//TODO: use matching instead of equality
			if contentType == core.HTML_CTYPE {
				rw.AddHeader(state.Ctx, CSP_HEADER_NAME, core.Str(server.defaultCSP.String()))
			}

			rw.WriteContentType(contentType)
			rw.BodyWriter().Write(v.Bytes)
		case core.Renderable:

			if !v.IsRecursivelyRenderable(state.Ctx, renderingConfig) { // get or create view
				logger.Print("result is not recursively renderable, attempt to get .view() for", req.Path)

				model, ok := v.(*core.Object)
				if !ok {
					if streamable, ok := v.(core.StreamSource); ok {
						value = streamable
						continue
					}

					rw.writeStatus(http.StatusNotFound)
					break loop
				}

				if !req.ParsedAcceptHeader.Match(core.HTML_CTYPE) && !req.ParsedAcceptHeader.Match(core.EVENT_STREAM_CTYPE) {
					rw.writeStatus(http.StatusNotAcceptable)
					return
				}

				//TODO: pause parallel identical requests then give them the created view

				properties := model.PropertyNames(state.Ctx)
				var renderFn core.Value
				for _, p := range properties {
					if p == "render" {
						renderFn = model.Prop(state.Ctx, "render")
					}
				}

				fn, ok := renderFn.(*core.InoxFunction)
				if !ok {
					if streamable, ok := v.(core.StreamSource); ok {
						value = streamable
						continue
					}

					rw.writeStatus(http.StatusNotFound)
					break loop
				}

				result, err := fn.Call(state, model, nil, nil)
				if err != nil {
					logger.Print("failed to create new view(): ", err.Error())
					rw.writeStatus(http.StatusInternalServerError)
					return
				} else {
					//TODO: prevent recursion
					value = result
					continue
				}
			} else {
				if req.Method != "GET" {
					rw.writeStatus(http.StatusMethodNotAllowed)
					return
				}

				if !req.ParsedAcceptHeader.Match(core.HTML_CTYPE) {
					rw.writeStatus(http.StatusNotAcceptable)
					return
				}

				rw.WriteContentType(core.HTML_CTYPE)
				rw.AddHeader(state.Ctx, CSP_HEADER_NAME, core.Str(server.defaultCSP.String()))

				_, err := core.Render(state.Ctx, rw.BodyWriter(), v, renderingConfig)
				if err != nil {
					logger.Print(err.Error())
				}
			}
		case core.StreamSource, core.ReadableStream:

			if req.AcceptAny() || !req.ParsedAcceptHeader.Match(core.EVENT_STREAM_CTYPE) {
				rw.writeStatus(http.StatusNotAcceptable)
				return
			}

			var stream core.ReadableStream
			switch s := v.(type) {
			case core.ReadableStream:
				stream = s
			case core.StreamSource:
				stream = s.Stream(state.Ctx, nil)
			}

			if !stream.ChunkDataType().Equal(state.Ctx, core.BYTESLICE_PATTERN, map[uintptr]uintptr{}, 0) {
				logger.Print("only byte streams can be streamed for now")
				rw.writeStatus(http.StatusNotAcceptable)
				return
			}

			state.Ctx.PromoteToLongLived()

			if err := pushByteStream(stream, h); err != nil {
				logger.Print(err)
				rw.writeStatus(http.StatusInternalServerError) //TODO: cancel context
				return
			}
		default:
			logger.Printf("routing mapping returned invalid value of type %T : %#v", v, v)
			rw.writeStatus(http.StatusInternalServerError)
		}
		break
	}
}
