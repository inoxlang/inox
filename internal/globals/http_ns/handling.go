package http_ns

import (
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"time"

	core "github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/dom_ns"
	"github.com/rs/zerolog"
)

const (
	DOM_EVENT_CTYPE = "dom/event"
)

var (
	DEFAULT_CSP, _ = dom_ns.NewCSPWithDirectives(nil)

	HANDLER_DISABLED_ARGS = []bool{true, true}
)

func isValidHandlerValue(val core.Value) bool {
	switch val.(type) {
	case *core.InoxFunction, *core.GoFunction, *core.Mapping:
		return true
	}
	return false
}

// a handlerFn is a middleware or the final handler
type handlerFn func(*HttpRequest, *HttpResponseWriter, *core.GlobalState, zerolog.Logger)

func createHandlerFunction(handlerValue core.Value, isMiddleware bool, server *HttpServer) (handler handlerFn) {

	//set value for handler based on provided arguments
	switch userHandler := handlerValue.(type) {
	case *core.InoxFunction:
		handler = func(req *HttpRequest, rw *HttpResponseWriter, handlerGlobalState *core.GlobalState, logger zerolog.Logger) {
			//call the Inox handler
			args := []core.Value{core.ValOf(rw), core.ValOf(req)}
			_, err := userHandler.Call(handlerGlobalState, nil, args, HANDLER_DISABLED_ARGS)

			if err != nil {
				logger.Print(err)
			}
		}
	case *core.GoFunction:
		handler = func(req *HttpRequest, rw *HttpResponseWriter, handlerGlobalState *core.GlobalState, logger zerolog.Logger) {
			//call the Golang handler
			args := []any{rw, req}

			_, err := userHandler.Call(args, handlerGlobalState, nil, false, false)

			if err != nil {
				logger.Print(err)
			}
		}
	case *core.Mapping:
		routing := userHandler
		//if a routing Mapping is provided we compute a value by passing the request's path to the Mapping.
		handler = func(req *HttpRequest, rw *HttpResponseWriter, state *core.GlobalState, logger zerolog.Logger) {
			path := req.Path

			value := routing.Compute(state.Ctx, path)
			if value == nil {
				logger.Print("routing mapping returned Go nil")
				rw.writeStatus(http.StatusNotFound)
				return
			}

			respondWithMappingResult(handlingArguments{value, req, rw, state, server, logger, isMiddleware})
		}
	default:
		panic(core.ErrUnreachable)

	}

	return handler
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
	}

	switch req.Method {
	case "GET", "HEAD":
		switch {
		case req.AcceptAny():
			break
		case req.ParsedAcceptHeader.Match(core.IXON_CTYPE):
			if !req.IsGetOrHead() {
				rw.writeStatus(http.StatusMethodNotAllowed)
				return
			}

			config := &core.ReprConfig{}

			if !value.HasRepresentation(map[uintptr]int{}, config) {
				rw.writeStatus(http.StatusNotAcceptable)
				return
			}

			rw.WriteContentType(core.IXON_CTYPE)
			value.WriteRepresentation(state.Ctx, rw.BodyWriter(), map[uintptr]int{}, config)
			return

		case req.ParsedAcceptHeader.Match(core.JSON_CTYPE):
			if !req.IsGetOrHead() {
				rw.writeStatus(http.StatusMethodNotAllowed)
				return
			}

			config := &core.ReprConfig{}

			if !value.HasJSONRepresentation(map[uintptr]int{}, config) {
				rw.writeStatus(http.StatusNotAcceptable)
				return
			}

			rw.WriteContentType(core.JSON_CTYPE)
			value.WriteJSONRepresentation(state.Ctx, rw.BodyWriter(), map[uintptr]int{}, config)
			return
		default:
			break
		}
	case "PATCH":
		switch {
		case req.ContentType.MatchText(DOM_EVENT_CTYPE):
			break // handled further below
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
		default:
			rw.writeStatus(http.StatusMethodNotAllowed)
			return
		}
	case "POST":
		rw.writeStatus(http.StatusMethodNotAllowed)
		return
	default:
		rw.writeStatus(http.StatusMethodNotAllowed)
		return
	}

	// rendering | event stream | dom event forwarding
loop:
	for {
		switch v := value.(type) {
		//values
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
			default:
				logger.Print("unknwon identifier " + string(v))
				rw.writeStatus(http.StatusNotFound)
			}

		case core.NilT, nil:
			logger.Print("nil result")
			rw.writeStatus(http.StatusNotFound)
			return

		case core.StringLike:
			if req.Method != "GET" {
				rw.writeStatus(http.StatusMethodNotAllowed)
				return
			}

			if !req.ParsedAcceptHeader.Match(core.PLAIN_TEXT_CTYPE) {
				rw.writeStatus(http.StatusNotAcceptable)
				return
			}

			//TODO: replace non printable characters
			escaped := html.EscapeString(v.GetOrBuildString())

			rw.WritePlainText(h.state.Ctx, &core.ByteSlice{Bytes: []byte(escaped)})
		case *core.ByteSlice:
			if req.Method != "GET" {
				rw.writeStatus(http.StatusMethodNotAllowed)
				return
			}

			contentType := string(v.ContentType())
			if !req.ParsedAcceptHeader.Match(contentType) {
				rw.writeStatus(http.StatusNotAcceptable)
				return
			}

			//TODO: use matching instead of equality
			if contentType == core.HTML_CTYPE {
				rw.AddHeader(state.Ctx, dom_ns.CSP_HEADER_NAME, core.Str(server.defaultCSP.String()))
			}

			rw.WriteContentType(contentType)
			rw.BodyWriter().Write(v.Bytes)
		case core.Renderable:

			if !v.IsRecursivelyRenderable(state.Ctx, renderingConfig) { // get or create view
				logger.Print("result is not renderable, attempt to get .view() for", req.Path)

				model, ok := v.(*core.Object)
				if !ok {
					if streamable, ok := v.(core.StreamSource); ok {
						value = streamable
						continue
					}

					rw.writeStatus(http.StatusNotFound)
					break loop
				}

				if !req.ParsedAcceptHeader.Match(core.HTML_CTYPE) && !req.ParsedAcceptHeader.Match(core.EVENT_STREAM_CTYPE) &&
					!req.ContentType.MatchText(DOM_EVENT_CTYPE) {
					rw.writeStatus(http.StatusNotAcceptable)
					return
				}

				//TODO: pause parallel identical requests then give them the created view
				view, ok := getOrCreateView(model, h)
				if ok {
					value = view //attempt to render with view as value
					continue
				}

				if streamable, ok := v.(core.StreamSource); ok {
					value = streamable
					continue
				}

				break loop
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
				rw.AddHeader(state.Ctx, dom_ns.CSP_HEADER_NAME, core.Str(server.defaultCSP.String()))

				_, err := core.Render(state.Ctx, rw.BodyWriter(), v, renderingConfig)
				if err != nil {
					logger.Print(err.Error())
				}
			}
		case *dom_ns.View:
			view := v
			switch req.Method {
			case "GET":
				switch {
				case req.ParsedAcceptHeader.Match(core.HTML_CTYPE):
					rw.WriteContentType(core.HTML_CTYPE)
					rw.AddHeader(state.Ctx, dom_ns.CSP_HEADER_NAME, core.Str(server.defaultCSP.String()))

					_, err := v.Node().Render(state.Ctx, rw.BodyWriter(), renderingConfig)
					if err != nil {
						logger.Print(err.Error())
					}

				case req.ParsedAcceptHeader.Match(core.EVENT_STREAM_CTYPE):

					if err := pushViewUpdates(v, h); err != nil {
						logger.Print(err)
						rw.writeStatus(http.StatusInternalServerError)
						return
					}
				default:
					rw.writeStatus(http.StatusNotAcceptable)
					return
				}
			case "PATCH":
				if !req.ContentType.MatchText(DOM_EVENT_CTYPE) {
					rw.writeStatus(http.StatusBadRequest)
					return
				}

				bytes, err := req.Body.ReadAllBytes()
				if err != nil {
					logger.Print(err)
					rw.writeStatus(http.StatusBadRequest)
					return
				}

				var unmarshalled any

				if err := json.Unmarshal(bytes, &unmarshalled); err != nil {
					logger.Print("failed ton parse DOM event:", err)
					rw.writeStatus(http.StatusBadRequest)
					return
				}

				data := core.ConvertJSONValToInoxVal(state.Ctx, unmarshalled, true)
				eventData, ok := data.(*core.Record)
				if !ok {
					logger.Print("DOM event data should be a record")
					rw.writeStatus(http.StatusBadRequest)
					return
				} else {
					logger.Print("dom event received")
				}

				view.SendDOMEventToForwader(state.Ctx, eventData, time.Now())
				return
			default:
				rw.writeStatus(http.StatusMethodNotAllowed)
				return
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

func getOrCreateView(model *core.Object, args handlingArguments) (view *dom_ns.View, viewOk bool) {
	req := args.req
	rw := args.rw
	state := args.state
	logger := args.logger

	renderFn := model.Prop(state.Ctx, "render")

	sessionView, found, set := req.Session.GetOrSetView(state.Ctx, req.Path, func() *dom_ns.View {

		if renderFn == nil {
			rw.writeStatus(http.StatusNotFound)
			return nil
		}

		fn, ok := renderFn.(*core.InoxFunction)
		if !ok {
			rw.writeStatus(http.StatusNotFound)
			return nil
		}

		html, err := fn.Call(state, model, nil, nil)
		if err != nil {
			logger.Print("failed to create new view(): ", err.Error())
			rw.writeStatus(http.StatusInternalServerError)
		} else {
			//TODO: check if is error like result

			switch h := html.(type) {
			case *dom_ns.Node:
				view = dom_ns.NewView(state.Ctx, req.Path, model, h)
				state.ProposeSystemGraph(h, fmt.Sprintf("root node for view of %s", req.Path))
				viewOk = true
				state.Ctx.PromoteToLongLived()
				return view
			default:
				rw.writeStatus(http.StatusNotAcceptable)
			}
		}

		return nil
	})

	if found && sessionView.ModelIs(state.Ctx, model) {
		logger.Print("view found in session for", req.Path)
		view = sessionView
		viewOk = true
		return
	}

	if set {
		logger.Print("new view created for", req.Path)
		view = sessionView
		viewOk = true
		return
	}

	return nil, false
}
