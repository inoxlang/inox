package http_ns

import (
	"html"
	"maps"
	"net/http"

	"slices"

	"github.com/inoxlang/inox/internal/core"
	jsoniter "github.com/inoxlang/inox/internal/jsoniter"
	"github.com/inoxlang/inox/internal/mimeconsts"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/rs/zerolog"
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

// addHandlerFunction creates a function of type handlerFn from handlerValue and updates the server.lastHandlerFn or server.middlewares.
// addHandlerFunction also sets server.api & server.preparedModules.
func addHandlerFunction(handlerValue core.Value, isMiddleware bool, server *HttpsServer) error {

	//set value for handler based on provided arguments
	switch userHandler := handlerValue.(type) {
	case *core.InoxFunction:
		handler := func(req *HttpRequest, rw *HttpResponseWriter, handlerGlobalState *core.GlobalState) {
			//add parent context's patterns
			serverCtx := server.state.Ctx
			for k, v := range serverCtx.GetNamedPatterns() {
				if handlerGlobalState.Ctx.ResolveNamedPattern(k) == nil {
					handlerGlobalState.Ctx.AddNamedPattern(k, v)
				}
			}
			for k, v := range serverCtx.GetPatternNamespaces() {
				if handlerGlobalState.Ctx.ResolvePatternNamespace(k) == nil {
					handlerGlobalState.Ctx.AddPatternNamespace(k, v)
				}
			}

			//call the Inox handler
			args := []core.Value{core.ValOf(rw), core.ValOf(req)}
			_, err := userHandler.Call(handlerGlobalState, nil, args, HANDLER_DISABLED_ARGS)

			if err != nil {
				handlerGlobalState.Logger.Print(err)
			}
		}

		if isMiddleware {
			server.middlewares = append(server.middlewares, handler)
		} else {
			server.lastHandlerFn = handler
			server.api = newEmptyAPI()
		}
	case *core.GoFunction:
		handler := func(req *HttpRequest, rw *HttpResponseWriter, handlerGlobalState *core.GlobalState) {
			//call the Golang handler
			args := []any{rw, req}

			_, err := userHandler.Call(args, handlerGlobalState, nil, false, false)

			if err != nil {
				handlerGlobalState.Logger.Print(err)
			}
		}
		if isMiddleware {
			server.middlewares = append(server.middlewares, handler)
		} else {
			server.lastHandlerFn = handler
			server.api = newEmptyAPI()
		}
	case *core.Object:
		//filesystem routing

		var staticDir core.Path
		var dynamicDir core.Path

		propertyNames := userHandler.PropertyNames(server.state.Ctx)
		if slices.Contains(propertyNames, "static") {
			staticDir = userHandler.Prop(server.state.Ctx, "static").(core.Path)
		}
		if slices.Contains(propertyNames, "dynamic") {
			dynamicDir = userHandler.Prop(server.state.Ctx, "dynamic").(core.Path)
		}

		return addFilesystemRoutingHandler(server, staticDir, dynamicDir, isMiddleware)
	case *core.Mapping:
		routing := userHandler
		//if a routing Mapping is provided we compute a value by passing the request's path to the Mapping.
		handler := func(req *HttpRequest, rw *HttpResponseWriter, handlerGlobalState *core.GlobalState) {
			path := req.Path

			//add parent context's patterns
			serverCtx := server.state.Ctx
			for k, v := range serverCtx.GetNamedPatterns() {
				if handlerGlobalState.Ctx.ResolveNamedPattern(k) == nil {
					handlerGlobalState.Ctx.AddNamedPattern(k, v)
				}
			}
			for k, v := range serverCtx.GetPatternNamespaces() {
				if handlerGlobalState.Ctx.ResolvePatternNamespace(k) == nil {
					handlerGlobalState.Ctx.AddPatternNamespace(k, v)
				}
			}

			//compute the result
			value := routing.Compute(handlerGlobalState.Ctx, path)
			if value == nil {
				handlerGlobalState.Logger.Print("routing mapping returned Go nil")
				rw.writeStatus(http.StatusNotFound)
				return
			}

			respondWithMappingResult(handlingArguments{
				value:        value,
				req:          req,
				rw:           rw,
				state:        handlerGlobalState,
				server:       server,
				logger:       handlerGlobalState.Logger,
				isMiddleware: isMiddleware,
			})
		}
		if isMiddleware {
			server.middlewares = append(server.middlewares, handler)
		} else {
			server.lastHandlerFn = handler
			server.api = newEmptyAPI()
		}
	default:
		panic(core.ErrUnreachable)
	}

	return nil
}

type handlingArguments struct {
	value        core.Value
	req          *HttpRequest
	rw           *HttpResponseWriter
	state        *core.GlobalState
	server       *HttpsServer
	logger       zerolog.Logger
	scriptsNonce string //optional
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
	renderingConfig := core.RenderingInput{Mime: mimeconsts.HTML_CTYPE}

	statusIfAccepted := http.StatusOK

	switch v := value.(type) {
	case *core.InoxFunction: // if inox handler we call it and return
		args := []core.Value{core.ValOf(rw), core.ValOf(req)}
		_, err := v.Call(state, nil, args, HANDLER_DISABLED_ARGS)

		if err != nil {
			logger.Print("error when calling returned inox function:", err)
		}
		return
	case Status:
		rw.writeStatus(int(v.code))
		return
	case StatusCode:
		rw.writeStatus(int(v))
		return
	case *HttpResult:
		maps.Copy(rw.rw.Header(), v.headers)
		statusIfAccepted = int(v.status)

		//use the value inside the result
		value = v.value
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
	case req.ParsedAcceptHeader.Match(mimeconsts.IXON_CTYPE):
		config := &core.ReprConfig{}

		serializable, ok := value.(core.Serializable)
		if !ok {
			rw.writeStatus(http.StatusNotAcceptable)
			return
		}

		//finalize and send headers
		rw.WriteContentType(mimeconsts.IXON_CTYPE)
		rw.writeStatus(statusIfAccepted)

		serializable.WriteRepresentation(state.Ctx, rw.BodyWriter(), config, 0)
		return
	case req.ParsedAcceptHeader.Match(mimeconsts.JSON_CTYPE):
		config := core.JSONSerializationConfig{
			ReprConfig: &core.ReprConfig{},
		}

		serializable, ok := value.(core.Serializable)
		if !ok {
			rw.writeStatus(http.StatusNotAcceptable)
			return
		}

		//finalize and send headers
		rw.WriteContentType(mimeconsts.JSON_CTYPE)
		rw.writeStatus(statusIfAccepted)

		//write value as JSON
		stream := jsoniter.NewStream(jsoniter.ConfigDefault, rw.BodyWriter(), 0)
		serializable.WriteJSONRepresentation(state.Ctx, stream, config, 0)
		stream.Flush()
		return
	}

	switch req.Method {
	case "GET", "HEAD", "OPTIONS":
		break
	case "POST", "PATCH":
		switch {
		case req.ContentType.MatchText(mimeconsts.APP_OCTET_STREAM_CTYPE):
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
		//TODO:
		// https://developer.mozilla.org/en-US/docs/web/http/status/405:
		// The server must generate an Allow header field in a 405 status code response.
		// The field must contain a list of methods that the target resource currently supports.
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
			if !req.ParsedAcceptHeader.Match(mimeconsts.PLAIN_TEXT_CTYPE) {
				rw.writeStatus(http.StatusNotAcceptable)
				return
			}

			//TODO: replace non printable characters
			escaped := html.EscapeString(v.GetOrBuildString())

			//finalize and send headers
			rw.WriteContentType(mimeconsts.PLAIN_TEXT_CTYPE)
			rw.writeStatus(statusIfAccepted)

			//write body
			rw.BodyWriter().Write(utils.StringAsBytes(escaped))
		case *core.ByteSlice:
			contentType := string(v.ContentType())
			if !req.ParsedAcceptHeader.Match(contentType) {
				rw.writeStatus(http.StatusNotAcceptable)
				return
			}
			//finalize and send headers
			if contentType == mimeconsts.HTML_CTYPE { //TODO: use matching instead of equality
				headerValue := server.defaultCSP.HeaderValue(CSPHeaderValueParams{ScriptsNonce: h.scriptsNonce})
				rw.AddHeader(state.Ctx, CSP_HEADER_NAME, core.Str(headerValue))
			}
			rw.WriteContentType(contentType)
			rw.writeStatus(statusIfAccepted)

			//write body
			rw.BodyWriter().Write(v.UnderlyingBytes())
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

				if !req.ParsedAcceptHeader.Match(mimeconsts.HTML_CTYPE) && !req.ParsedAcceptHeader.Match(mimeconsts.EVENT_STREAM_CTYPE) {
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
				if !req.ParsedAcceptHeader.Match(mimeconsts.HTML_CTYPE) {
					rw.writeStatus(http.StatusNotAcceptable)
					return
				}

				//finalize and send headers
				rw.WriteContentType(mimeconsts.HTML_CTYPE)

				cspHeaderValue := core.Str(server.defaultCSP.HeaderValue(CSPHeaderValueParams{ScriptsNonce: h.scriptsNonce}))
				rw.AddHeader(state.Ctx, CSP_HEADER_NAME, cspHeaderValue)
				rw.writeStatus(statusIfAccepted)

				//write body
				_, err := core.Render(state.Ctx, rw.BodyWriter(), v, renderingConfig)
				if err != nil {
					logger.Print(err.Error())
				}
			}
		case core.StreamSource, core.ReadableStream:

			if req.AcceptAny() || !req.ParsedAcceptHeader.Match(mimeconsts.EVENT_STREAM_CTYPE) {
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
