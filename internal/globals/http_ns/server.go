package http_ns

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	core "github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/mimeconsts"
	"github.com/rs/zerolog"
	"golang.org/x/exp/maps"

	http_ns_symb "github.com/inoxlang/inox/internal/globals/http_ns/symbolic"

	"github.com/inoxlang/inox/internal/permkind"
)

const (
	DEFAULT_HTTP_SERVER_READ_HEADER_TIMEOUT = 3 * time.Second
	DEFAULT_HTTP_SERVER_READ_TIMEOUT        = DEFAULT_HTTP_SERVER_READ_HEADER_TIMEOUT + 8*time.Second
	DEFAULT_HTTP_SERVER_WRITE_TIMEOUT       = 2 * (DEFAULT_HTTP_SERVER_READ_TIMEOUT - DEFAULT_HTTP_SERVER_READ_HEADER_TIMEOUT)
	DEFAULT_HTTP_SERVER_MAX_HEADER_BYTES    = 1 << 12
	DEFAULT_HTTP_SERVER_TX_TIMEOUT          = 20 * time.Second
	SSE_STREAM_WRITE_TIMEOUT                = 500 * time.Second

	HTTP_SERVER_STARTING_WAIT_TIME        = 5 * time.Millisecond
	HTTP_SERVER_GRACEFUL_SHUTDOWN_TIMEOUT = 5 * time.Second

	NO_HANDLER_PLACEHOLDER_MESSAGE = "hello"

	HANDLING_DESC_MIDDLEWARES_PROPNAME = "middlewares"
	HANDLING_DESC_ROUTING_PROPNAME     = "routing"
	HANDLING_DESC_DEFAULT_CSP_PROPNAME = "default-csp"
	HANDLING_DESC_CERTIFICATE_PROPNAME = "certificate"
	HANDLING_DESC_KEY_PROPNAME         = "key"

	HANDLING_DESC_DEFAULT_LIMITS_PROPNAME = "default-limits"
	HANDLING_DESC_MAX_LIMITS_PROPNAME     = "max-limits"

	HTTP_SERVER_SRC_PATH = "/http/server"
)

var (
	ErrHandlerNotSharable = errors.New("handler is not sharable")

	HTTP_ROUTING_SYMB_OBJ = symbolic.NewInexactObject(map[string]symbolic.Serializable{
		"static":  symbolic.ANY_ABS_DIR_PATH,
		"dynamic": symbolic.ANY_ABS_DIR_PATH,
	}, map[string]struct{}{
		"static": {}, "dynamic": {},
	}, nil)

	SYMBOLIC_HANDLING_DESC = symbolic.NewInexactObject(map[string]symbolic.Serializable{
		HANDLING_DESC_ROUTING_PROPNAME: symbolic.AsSerializableChecked(symbolic.NewMultivalue(
			symbolic.ANY_INOX_FUNC,
			symbolic.NewMapping(),
			HTTP_ROUTING_SYMB_OBJ,
		)),
		HANDLING_DESC_MIDDLEWARES_PROPNAME:    symbolic.ANY_SERIALIZABLE_ITERABLE,
		HANDLING_DESC_DEFAULT_CSP_PROPNAME:    http_ns_symb.ANY_CSP,
		HANDLING_DESC_CERTIFICATE_PROPNAME:    symbolic.ANY_STR_LIKE,
		HANDLING_DESC_KEY_PROPNAME:            symbolic.ANY_SECRET,
		HANDLING_DESC_DEFAULT_LIMITS_PROPNAME: symbolic.ANY_OBJ,
		HANDLING_DESC_MAX_LIMITS_PROPNAME:     symbolic.ANY_OBJ,
	}, map[string]struct{}{
		//optional entries
		HANDLING_DESC_MIDDLEWARES_PROPNAME:    {},
		HANDLING_DESC_DEFAULT_CSP_PROPNAME:    {},
		HANDLING_DESC_CERTIFICATE_PROPNAME:    {},
		HANDLING_DESC_KEY_PROPNAME:            {},
		HANDLING_DESC_DEFAULT_LIMITS_PROPNAME: {},
		HANDLING_DESC_MAX_LIMITS_PROPNAME:     {},
	}, nil)

	NEW_SERVER_TWO_PARAM_NAMES = []string{"host", "handling"}
)

// HttpServer implements the GoValue interface.
type HttpServer struct {
	host          core.Host
	wrappedServer *http.Server
	lock          sync.RWMutex

	endChan      chan struct{}
	state        *core.GlobalState
	serverLogger zerolog.Logger

	lastHandlerFn handlerFn
	middlewares   []handlerFn

	sseServer      *SseServer
	defaultCSP     *ContentSecurityPolicy
	securityEngine *securityEngine
	api            *API
	//preparedModules *preparedModule //mostly used during invocation of handler modules

	defaultLimits          map[string]core.Limit //readonly
	maxHandlerModuleLimits map[string]core.Limit //readonly
}

func NewHttpServer(ctx *core.Context, host core.Host, args ...core.Value) (*HttpServer, error) {
	ctxLogger := *ctx.Logger()
	_server := &HttpServer{
		state:      ctx.GetClosestState(),
		defaultCSP: DEFAULT_CSP,
	}

	if _server.state == nil {
		return nil, errors.New("cannot create server: context's associated state is nil")
	}

	addr, userProvidedCert, userProvidedKey, userProvidedHandler, handlerValProvided, middlewares,
		defaultLimits, maxLimits, argErr := readHttpServerArgs(ctx, _server, host, args...)

	if argErr != nil {
		return nil, argErr
	}

	_server.maxHandlerModuleLimits = maxLimits
	_server.defaultLimits = defaultLimits

	//create logger and security engine
	{
		logSrc := HTTP_SERVER_SRC_PATH + "/" + addr
		_server.serverLogger = ctxLogger.With().Str(core.SOURCE_LOG_FIELD_NAME, logSrc).Logger()
		_server.securityEngine = newSecurityEngine(ctxLogger, logSrc)
	}

	//create middleware functions + last handler function
	if handlerValProvided {
		err := addHandlerFunction(userProvidedHandler, false, _server)
		if err != nil {
			return nil, err
		}
	} else {
		//we set a default handler that writes NO_HANDLER_PLACEHOLDER_MESSAGE
		_server.lastHandlerFn = func(r *HttpRequest, rw *HttpResponseWriter, state *core.GlobalState) {
			rw.rw.Write([]byte(NO_HANDLER_PLACEHOLDER_MESSAGE))
		}
	}

	for _, val := range middlewares {
		err := addHandlerFunction(val, true, _server)
		if err != nil {
			return nil, err
		}
	}

	// create the http.HandlerFunc that will call lastHandlerFn & middlewares
	topHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serverLogger := _server.serverLogger

		//create the Inox values for the request and the response writer
		req, err := NewServerSideRequest(r, serverLogger, _server)
		if err != nil {
			serverLogger.Print(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		rw := NewResponseWriter(req, w, serverLogger)

		debugger, _ := _server.state.Debugger.Load().(*core.Debugger)
		if debugger != nil {
			debugger.ControlChan() <- core.DebugCommandInformAboutSecondaryEvent{
				Event: core.IncomingMessageReceivedEvent{
					MessageType: "http/request",
					Url:         string(req.URL),
				},
			}
		}

		// check that path does not contain '..' elements.
		// use same logic as containsDotDot in stdlib net/http/fs.go

		isSlashRune := func(r rune) bool { return r == '/' || r == '\\' }

		for _, ent := range strings.FieldsFunc(r.URL.Path, isSlashRune) {
			if ent == ".." {
				rw.writeStatus(http.StatusBadRequest)
				return
			}
		}

		// rate limiting & more

		if _server.securityEngine.rateLimitRequest(req, rw) {
			rw.writeStatus(http.StatusTooManyRequests)
			return
		}

		//create a global state for handling the request
		handlerCtx := core.NewContext(core.ContextConfig{
			Permissions:          ctx.GetGrantedPermissions(),
			ForbiddenPermissions: ctx.GetForbiddenPermissions(),
			Limits:               maps.Values(defaultLimits),
			ParentContext:        ctx,
			Filesystem:           ctx.GetFileSystem(),
		})

		defer handlerCtx.CancelIfShortLived()

		if !req.ParsedAcceptHeader.Match(mimeconsts.EVENT_STREAM_CTYPE) {
			tx := core.StartNewTransaction(handlerCtx, core.Option{
				Name:  core.TX_TIMEOUT_OPTION_NAME,
				Value: core.Duration(DEFAULT_HTTP_SERVER_TX_TIMEOUT),
			})
			defer tx.Commit(ctx)
		}

		//transaction is cleaned up during context cancelation, so no need to defer a rollback

		handlerGlobalState := core.NewGlobalState(handlerCtx)
		handlerGlobalState.Logger = _server.state.Logger
		handlerGlobalState.Out = _server.state.Out
		handlerGlobalState.Module = _server.state.Module
		handlerGlobalState.MainState = _server.state.MainState
		handlerGlobalState.Manifest = _server.state.Manifest
		handlerGlobalState.Databases = _server.state.Databases
		handlerGlobalState.SystemGraph = _server.state.SystemGraph
		handlerGlobalState.OutputFieldsInitialized.Store(true)

		//

		if req.NewSession {
			addSessionIdCookie(rw, req.Session.Id)
		}

		defer rw.FinalLog()

		//call middlewares & handler

		for _, fn := range _server.middlewares {
			fn(req, rw, handlerGlobalState)
			if rw.finished {
				return
			}
		}

		//TODO: make sure memory allocated by + resources acquired by middlewares are released

		_server.lastHandlerFn(req, rw, handlerGlobalState)
	})

	//create a stdlib http Server
	config := GolangHttpServerConfig{
		Addr:    addr,
		Handler: topHandler,
	}
	if userProvidedCert != "" {
		config.PemEncodedCert = userProvidedCert
	}

	if userProvidedKey != nil {
		config.PemEncodedKey = userProvidedKey.StringValue().GetOrBuildString()
	}

	server, err := NewGolangHttpServer(ctx, config)
	if err != nil {
		return nil, err
	}

	_server.wrappedServer = server
	_server.endChan = make(chan struct{}, 1)

	//listen and serve in a goroutine
	go func() {
		defer func() {
			recover()
			_server.endChan <- struct{}{}
		}()
		_server.serverLogger.Info().Msg("serve " + addr)

		err := server.ListenAndServeTLS("", "")
		if err != nil {
			_server.serverLogger.Print(err)
		}
	}()

	//ungracefully stop server after context is done
	go func() {
		defer func() {
			recover()
			_server.endChan <- struct{}{}
		}()
		defer _server.serverLogger.Info().Msg("server (" + addr + ") is now closed")

		<-ctx.Done()
		_server.ImmediatelyClose(ctx)
	}()

	time.Sleep(HTTP_SERVER_STARTING_WAIT_TIME)

	return _server, nil
}

func (serv *HttpServer) Host(name string) core.Host {
	return serv.host
}

func (serv *HttpServer) getOrCreateStream(id string) (*multiSubscriptionSSEStream, *SseServer, error) {
	serv.lock.Lock()
	defer serv.lock.Unlock()

	if serv.sseServer == nil {
		serv.sseServer = NewSseServer()
	}

	stream := serv.sseServer.getStream(id)
	if stream == nil {
		stream = serv.sseServer.CreateStream(id)
	}

	return stream, serv.sseServer, nil
}

func (serv *HttpServer) GetGoMethod(name string) (*core.GoFunction, bool) {
	switch name {
	case "wait_closed":
		return core.WrapGoMethod(serv.WaitClosed), true
	case "close":
		return core.WrapGoMethod(serv.Close), true
	}
	return nil, false
}

func (s *HttpServer) Prop(ctx *core.Context, name string) core.Value {
	method, ok := s.GetGoMethod(name)
	if !ok {
		panic(core.FormatErrPropertyDoesNotExist(name, s))
	}
	return method
}

func (*HttpServer) SetProp(ctx *core.Context, name string, value core.Value) error {
	return core.ErrCannotSetProp
}

func (*HttpServer) PropertyNames(ctx *core.Context) []string {
	return http_ns_symb.HTTP_SERVER_PROPNAMES
}

func (serv *HttpServer) WaitClosed(ctx *core.Context) {
	<-serv.endChan
}

func (serv *HttpServer) ImmediatelyClose(ctx *core.Context) {
	serv.wrappedServer.Close()

	serv.lock.Lock()
	sse := serv.sseServer
	serv.lock.Unlock()

	if sse != nil {
		sse.Close()
	}
}

func (serv *HttpServer) Close(ctx *core.Context) {
	//we first close the event streams to prevent hanging during shutdown
	serv.lock.Lock()
	sse := serv.sseServer
	serv.lock.Unlock()

	if sse != nil {
		sse.Close()
		// wait a little ?
	}

	// gracefully shutdown the server
	timeoutCtx, cancel := context.WithTimeout(ctx, HTTP_SERVER_GRACEFUL_SHUTDOWN_TIMEOUT)
	defer cancel()
	serv.wrappedServer.Shutdown(timeoutCtx)
}

func newSymbolicHttpServer(ctx *symbolic.Context, host *symbolic.Host, args ...symbolic.Value) (*http_ns_symb.HttpServer, *symbolic.Error) {
	if !ctx.HasAPermissionWithKindAndType(permkind.Provide, permkind.HTTP_PERM_TYPENAME) {
		ctx.AddSymbolicGoFunctionWarning(HTTP_PROVIDE_PERM_MIGHT_BE_MISSING)
	}

	server := &http_ns_symb.HttpServer{}

	if len(args) == 0 {
		return server, nil
	}

	switch args[0].(type) {
	case *symbolic.InoxFunction:
	case *symbolic.GoFunction:
	case *symbolic.Mapping:
	case *symbolic.Object:
		ctx.SetSymbolicGoFunctionParameters(&[]symbolic.Value{
			symbolic.ANY_HOST,
			SYMBOLIC_HANDLING_DESC,
		}, NEW_SERVER_TWO_PARAM_NAMES)
	default:
		ctx.SetSymbolicGoFunctionParameters(&[]symbolic.Value{
			symbolic.ANY_HOST,
			symbolic.NewMultivalue(
				symbolic.ANY_INOX_FUNC,
				symbolic.NewMapping(),
				SYMBOLIC_HANDLING_DESC,
			),
		}, NEW_SERVER_TWO_PARAM_NAMES)
	}

	return server, nil
}
