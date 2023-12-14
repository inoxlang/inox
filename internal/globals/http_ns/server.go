package http_ns

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"runtime/debug"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/mimeconsts"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/rs/zerolog"
	"golang.org/x/exp/maps"

	"github.com/inoxlang/inox/internal/globals/fs_ns"
	http_ns_symb "github.com/inoxlang/inox/internal/globals/http_ns/symbolic"

	"github.com/inoxlang/inox/internal/core/permkind"
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

	HTTP_SERVER_SRC = "http/server"
)

var (
	ErrHandlerNotSharable            = errors.New("handler is not sharable")
	ErrCannotMutateInitializedServer = errors.New("cannot mutate initialized server")

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

	NEW_SERVER_SINGLE_PARAM_NAME = []string{"host"}
	NEW_SERVER_TWO_PARAM_NAMES   = []string{"host", "handling"}
)

// HttpsServer implements the GoValue interface.
type HttpsServer struct {
	host          core.Host
	wrappedServer *http.Server
	initialized   atomic.Bool
	lock          sync.RWMutex

	endChan       chan struct{}
	state         *core.GlobalState
	serverLogger  zerolog.Logger
	fsEventSource *fs_ns.FilesystemEventSource

	lastHandlerFn handlerFn
	middlewares   []handlerFn

	sseServer      *SseServer
	defaultCSP     *ContentSecurityPolicy
	securityEngine *securityEngine

	api     *API //An API is immutable but this field can be re-assigned.
	apiLock sync.Mutex

	//preparedModules *preparedModule //mostly used during invocation of handler modules

	defaultLimits map[string]core.Limit //readonly
	maxLimits     map[string]core.Limit //readonly
}

// NewHttpsServer creates a listening HTTPS server.
// The server's defaultLimits are constructed by merging the default request handling limits with the default-limits in arguments.
// The server's maxLimits are constructed by merging the default max request handling limits with the max-limits in arguments.
func NewHttpsServer(ctx *core.Context, host core.Host, args ...core.Value) (*HttpsServer, error) {
	_server := &HttpsServer{
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

	_server.maxLimits = maxLimits
	_server.defaultLimits = defaultLimits

	//create logger and security engine
	{
		logSrc := HTTP_SERVER_SRC + "/" + addr
		_server.serverLogger = ctx.NewChildLoggerForInternalSource(logSrc)

		securityLogSrc := ctx.NewChildLoggerForInternalSource(logSrc + "/sec")
		_server.securityEngine = newSecurityEngine(securityLogSrc)
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
		handlerGlobalState.LogLevels = _server.state.LogLevels
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
		Addr:                    addr,
		Handler:                 topHandler,
		PersistCreatedLocalCert: true,
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
	_server.initialized.Store(true)

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

func (serv *HttpsServer) Host(name string) core.Host {
	return serv.host
}

func (serv *HttpsServer) getOrCreateStream(id string) (*multiSubscriptionSSEStream, *SseServer, error) {
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

func (serv *HttpsServer) GetGoMethod(name string) (*core.GoFunction, bool) {
	switch name {
	case "wait_closed":
		return core.WrapGoMethod(serv.WaitClosed), true
	case "close":
		return core.WrapGoMethod(serv.Close), true
	}
	return nil, false
}

func (s *HttpsServer) Prop(ctx *core.Context, name string) core.Value {
	method, ok := s.GetGoMethod(name)
	if !ok {
		panic(core.FormatErrPropertyDoesNotExist(name, s))
	}
	return method
}

func (*HttpsServer) SetProp(ctx *core.Context, name string, value core.Value) error {
	return core.ErrCannotSetProp
}

func (*HttpsServer) PropertyNames(ctx *core.Context) []string {
	return http_ns_symb.HTTP_SERVER_PROPNAMES
}

func (serv *HttpsServer) WaitClosed(ctx *core.Context) {
	<-serv.endChan
}

func (serv *HttpsServer) ImmediatelyClose(ctx *core.Context) {
	serv.wrappedServer.Close()

	serv.lock.Lock()
	sse := serv.sseServer
	serv.lock.Unlock()

	if sse != nil {
		sse.Close()
	}
}

type idleFilesystemHandler struct {
	microtask    func(serverCtx *core.Context)
	watchedPaths []core.PathPattern
}

func (serv *HttpsServer) onIdleFilesystem(handler idleFilesystemHandler) {
	if serv.initialized.Load() {
		panic(ErrCannotMutateInitializedServer)
	}

	if len(handler.watchedPaths) == 0 {
		return
	}

	if serv.fsEventSource == nil {
		fls := serv.state.Ctx.GetFileSystem()
		//TODO: only watch .ix files and the /static/ folder.
		evs, err := fs_ns.NewEventSourceWithFilesystem(serv.state.Ctx, fls, core.PathPattern("/..."))
		if err != nil {
			panic(err)
		}
		serv.fsEventSource = evs
	}

	logger := serv.serverLogger
	serverCtx := serv.state.Ctx

	isRelevantEvent := func(e *core.Event) bool {
		fsEvent := e.SourceValue().(fs_ns.Event)
		if !fsEvent.IsStructureOrContentChange() {
			return false
		}

		for _, pathPattern := range handler.watchedPaths {
			if pathPattern.Test(nil, fsEvent.Path()) {
				return true
			}
		}

		return false
	}

	serv.fsEventSource.OnIDLE(core.IdleEventSourceHandler{
		MinimumLastEventAge: core.HARD_MINIMUM_LAST_EVENT_AGE,
		IsIgnoredEvent: func(e *core.Event) core.Bool {
			return !core.Bool(isRelevantEvent(e))
		},
		Microtask: func() {
			defer func() {
				e := recover()
				if e != nil {
					err := utils.ConvertPanicValueToError(e)
					err = fmt.Errorf("%w: %s", err, debug.Stack())
					logger.Err(err).Msg("error in on-idle microtask")
				}
			}()
			handler.microtask(serverCtx)
		},
	})
}

func (serv *HttpsServer) Close(ctx *core.Context) {
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

	if serv.fsEventSource != nil {
		serv.fsEventSource.Close()
	}
}

func newSymbolicHttpsServer(ctx *symbolic.Context, host *symbolic.Host, args ...symbolic.Value) (*http_ns_symb.HttpServer, *symbolic.Error) {
	if !ctx.HasAPermissionWithKindAndType(permkind.Provide, permkind.HTTP_PERM_TYPENAME) {
		ctx.AddSymbolicGoFunctionWarning(HTTP_PROVIDE_PERM_MIGHT_BE_MISSING)
	}

	symbolic.ANY_HOST_PATTERN.PropertyNames()

	server := &http_ns_symb.HttpServer{}

	if len(args) == 0 {
		if !symbolic.ANY_HTTPS_HOST_PATTERN.Test(host, symbolic.RecTestCallState{}) {
			ctx.SetSymbolicGoFunctionParameters(&[]symbolic.Value{symbolic.ANY_HTTPS_HOST}, NEW_SERVER_SINGLE_PARAM_NAME)
		}
		return server, nil
	}

	switch args[0].(type) {
	case *symbolic.InoxFunction:
	case *symbolic.GoFunction:
	case *symbolic.Mapping:
	case *symbolic.Object:
		ctx.SetSymbolicGoFunctionParameters(&[]symbolic.Value{
			symbolic.ANY_HTTPS_HOST,
			SYMBOLIC_HANDLING_DESC,
		}, NEW_SERVER_TWO_PARAM_NAMES)
	default:
		ctx.SetSymbolicGoFunctionParameters(&[]symbolic.Value{
			symbolic.ANY_HTTPS_HOST,
			symbolic.NewMultivalue(
				symbolic.ANY_INOX_FUNC,
				symbolic.NewMapping(),
				SYMBOLIC_HANDLING_DESC,
			),
		}, NEW_SERVER_TWO_PARAM_NAMES)
	}

	return server, nil
}
