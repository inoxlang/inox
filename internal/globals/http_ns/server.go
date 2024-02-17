package http_ns

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"runtime/debug"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/inoxlang/inox/internal/compressarch"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/mimeconsts"
	"github.com/inoxlang/inox/internal/netaddr"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/rs/zerolog"
	"golang.org/x/exp/maps"

	"github.com/inoxlang/inox/internal/globals/containers/common"
	"github.com/inoxlang/inox/internal/globals/containers/setcoll"
	symb_containers "github.com/inoxlang/inox/internal/globals/containers/symbolic"

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

	HANDLING_DESC_ROUTING_PROPNAME     = "routing"
	HANDLING_DESC_DEFAULT_CSP_PROPNAME = "default-csp"
	HANDLING_DESC_CERTIFICATE_PROPNAME = "certificate"
	HANDLING_DESC_KEY_PROPNAME         = "key"

	HANDLING_DESC_DEFAULT_LIMITS_PROPNAME = "default-limits"
	HANDLING_DESC_MAX_LIMITS_PROPNAME     = "max-limits"
	HANDLING_DESC_SESSIONS_PROPNAME       = "sessions"
	SESSIONS_DESC_COLLECTION_PROPNAME     = "collection"

	HTTP_SERVER_SRC = "http/server"

	SESSION_ID_PROPNAME = "id"
)

var (
	ErrHandlerNotSharable            = errors.New("handler is not sharable")
	ErrCannotMutateInitializedServer = errors.New("cannot mutate initialized server")

	HTTP_ROUTING_SYMB_OBJ = symbolic.NewInexactObject(map[string]symbolic.Serializable{
		"static":  symbolic.ANY_ABS_DIR_PATH,
		"dynamic": symbolic.ANY_ABS_DIR_PATH,
	}, map[string]struct{}{
		"static":  {},
		"dynamic": {},
	}, nil)

	SESSIONS_CONFIG_SYMB_OBJ = symbolic.NewInexactObject2(map[string]symbolic.Serializable{
		SESSIONS_DESC_COLLECTION_PROPNAME: symb_containers.NewSetWithPattern(symbolic.ANY_PATTERN, common.NewPropertyValueUniqueness(SESSION_ID_PROPNAME)),
	})

	SYMBOLIC_HANDLING_DESC = symbolic.NewInexactObject(map[string]symbolic.Serializable{
		HANDLING_DESC_ROUTING_PROPNAME: symbolic.AsSerializableChecked(symbolic.NewMultivalue(
			symbolic.ANY_INOX_FUNC,
			symbolic.NewMapping(),
			HTTP_ROUTING_SYMB_OBJ,
		)),
		HANDLING_DESC_DEFAULT_CSP_PROPNAME:    http_ns_symb.ANY_CSP,
		HANDLING_DESC_CERTIFICATE_PROPNAME:    symbolic.ANY_STR_LIKE,
		HANDLING_DESC_KEY_PROPNAME:            symbolic.ANY_SECRET,
		HANDLING_DESC_DEFAULT_LIMITS_PROPNAME: symbolic.ANY_OBJ,
		HANDLING_DESC_MAX_LIMITS_PROPNAME:     symbolic.ANY_OBJ,
		HANDLING_DESC_SESSIONS_PROPNAME:       SESSIONS_CONFIG_SYMB_OBJ,
	}, map[string]struct{}{
		//optional entries
		HANDLING_DESC_DEFAULT_CSP_PROPNAME:    {},
		HANDLING_DESC_CERTIFICATE_PROPNAME:    {},
		HANDLING_DESC_KEY_PROPNAME:            {},
		HANDLING_DESC_DEFAULT_LIMITS_PROPNAME: {},
		HANDLING_DESC_MAX_LIMITS_PROPNAME:     {},
		HANDLING_DESC_SESSIONS_PROPNAME:       {},
	}, nil)

	NEW_SERVER_SINGLE_PARAM_NAME = []string{"host"}
	NEW_SERVER_TWO_PARAM_NAMES   = []string{"host", "handling"}
)

// HttpsServer implements the GoValue interface.
type HttpsServer struct {
	listeningAddr core.Host
	wrappedServer *http.Server
	initialized   atomic.Bool
	lock          sync.RWMutex

	endChan        chan struct{}
	state          *core.GlobalState
	serverLogger   zerolog.Logger
	fsEventSource  *fs_ns.FilesystemEventSource
	fileCompressor *compressarch.FileCompressor

	lastHandlerFn handlerFn
	middlewares   []handlerFn

	sseServer      *SseServer
	defaultCSP     *ContentSecurityPolicy
	securityEngine *securityEngine

	api     *API //An API is immutable but this field can be re-assigned.
	apiLock sync.Mutex

	sessions *setcoll.Set //can be nil

	//preparedModules *preparedModule //mostly used during invocation of handler modules

	defaultLimits map[string]core.Limit //readonly
	maxLimits     map[string]core.Limit //readonly
}

// NewHttpsServer creates a listening HTTPS server.
// The server's defaultLimits are constructed by merging the default request handling limits with the default-limits in arguments.
// The server's maxLimits are constructed by merging the default max request handling limits with the max-limits in arguments.
func NewHttpsServer(ctx *core.Context, host core.Host, args ...core.Value) (*HttpsServer, error) {
	server := &HttpsServer{
		state:          ctx.GetClosestState(),
		defaultCSP:     DEFAULT_CSP,
		fileCompressor: compressarch.NewFileCompressor(),
	}

	if server.state == nil {
		return nil, errors.New("cannot create server: context's associated state is nil")
	}

	params, argErr := determineHttpServerParams(ctx, server, host, args...)

	if argErr != nil {
		return nil, argErr
	}

	server.maxLimits = params.maxLimits
	server.defaultLimits = params.defaultLimits
	server.listeningAddr = params.effectiveListeningAddrHost
	server.sessions = params.sessions
	if server.sessions != nil {
		server.sessions.Share(server.state)
	}

	//create logger and security engine
	{
		logSrc := HTTP_SERVER_SRC + "/" + params.effectiveAddr
		server.serverLogger = ctx.NewChildLoggerForInternalSource(logSrc)

		securityLogSrc := ctx.NewChildLoggerForInternalSource(logSrc + "/sec")
		server.securityEngine = newSecurityEngine(securityLogSrc)
	}

	//last handler function
	if params.handlerValProvided {
		err := addHandlerFunction(params.userProvidedHandler, false, server)
		if err != nil {
			return nil, err
		}
	} else {
		//we set a default handler that writes NO_HANDLER_PLACEHOLDER_MESSAGE
		server.lastHandlerFn = func(r *Request, rw *ResponseWriter, state *core.GlobalState) {
			rw.DetachRespWriter().Write([]byte(NO_HANDLER_PLACEHOLDER_MESSAGE))
		}
	}

	// create the http.HandlerFunc that will call lastHandlerFn & middlewares
	topHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serverLogger := server.serverLogger

		//create the Inox values for the request and the response writer
		req, err := NewServerSideRequest(r, serverLogger, server)
		if err != nil {
			serverLogger.Print(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		rw := NewResponseWriter(req, w, serverLogger)

		debugger, _ := server.state.Debugger.Load().(*core.Debugger)
		if debugger != nil {
			debugger.ControlChan() <- core.DebugCommandInformAboutSecondaryEvent{
				Event: core.IncomingMessageReceivedEvent{
					MessageType: "http/request",
					Url:         string(req.URL),
				},
			}
		}

		// rate limiting & more

		if server.securityEngine.rateLimitRequest(req, rw) {
			rw.writeHeaders(http.StatusTooManyRequests)
			return
		}

		//create a global state for handling the request
		handlerCtx := core.NewContext(core.ContextConfig{
			Permissions:          ctx.GetGrantedPermissions(),
			ForbiddenPermissions: ctx.GetForbiddenPermissions(),
			Limits:               maps.Values(params.defaultLimits),
			ParentContext:        ctx,
			Filesystem:           ctx.GetFileSystem(),
		})

		defer handlerCtx.CancelIfShortLived()

		if req.AcceptAny() || !req.ParsedAcceptHeader.Match(mimeconsts.EVENT_STREAM_CTYPE) {
			//Create a transaction if the client does not except an event stream.

			options := []core.Option{{
				Name:  core.TX_TIMEOUT_OPTION_NAME,
				Value: core.Duration(DEFAULT_HTTP_SERVER_TX_TIMEOUT),
			}}

			var tx *core.Transaction
			if req.IsGetOrHead() {
				tx = core.StartNewReadonlyTransaction(handlerCtx, options...)
			} else {
				tx = core.StartNewTransaction(handlerCtx, options...)
			}

			defer tx.Commit(ctx)
		}

		//transaction is cleaned up during context cancelation, so no need to defer a rollback

		handlerGlobalState := core.NewGlobalState(handlerCtx)
		handlerGlobalState.Logger = server.state.Logger
		handlerGlobalState.LogLevels = server.state.LogLevels
		handlerGlobalState.Out = server.state.Out
		handlerGlobalState.Module = server.state.Module
		handlerGlobalState.MainState = server.state.MainState
		handlerGlobalState.Manifest = server.state.Manifest
		handlerGlobalState.Databases = server.state.Databases
		handlerGlobalState.SystemGraph = server.state.SystemGraph
		handlerGlobalState.OutputFieldsInitialized.Store(true)

		//Get session
		session, err := server.getSession(handlerCtx, req)
		if err == nil {
			req.Session = session
			handlerCtx.PutUserData(SESSION_CTX_DATA_KEY, session)
		}

		defer func() {
			e := recover()
			if e != nil {
				err := utils.ConvertPanicValueToError(e)
				err = fmt.Errorf("%w: %s", err, debug.Stack())
				serverLogger.Err(err).Send()
			}
		}()

		defer rw.FinalLog()

		//call middlewares & handler

		for _, fn := range server.middlewares {
			fn(req, rw, handlerGlobalState)
			if rw.finished {
				return
			}
		}

		//TODO: make sure memory allocated by + resources acquired by middlewares are released

		server.lastHandlerFn(req, rw, handlerGlobalState)
	})

	//create a stdlib http Server
	config := GolangHttpServerConfig{
		Addr:                    params.effectiveAddr,
		Handler:                 topHandler,
		PersistCreatedLocalCert: true,
		AllowSelfSignedCertCreationEvenIfExposed: isLocalhostOr127001Addr(params.effectiveAddr) ||
			(params.exposingAllowed && isBindAllAddress(params.effectiveAddr)),
	}
	if params.certificate != "" {
		config.PemEncodedCert = params.certificate
	}

	if params.certKey != nil {
		config.PemEncodedKey = params.certKey.StringValue().GetOrBuildString()
	}

	goServer, err := NewGolangHttpServer(ctx, config)
	if err != nil {
		return nil, err
	}

	server.wrappedServer = goServer
	server.endChan = make(chan struct{}, 1)
	server.initialized.Store(true)

	//listen and serve in a goroutine
	go func() {
		defer func() {
			recover()
			server.endChan <- struct{}{}
		}()

		//log

		ips, err := netaddr.GetGlobalUnicastIPs()
		if err != nil {
			server.serverLogger.Err(err).Send()
			return
		}

		urls := []string{"https://localhost:" + params.port}
		if !isLocalhostOr127001Addr(params.effectiveAddr) {
			urls = append(urls, utils.FilterMapSlice(ips, func(e net.IP) (string, bool) {
				if e.To4() == nil {
					return "", false
				}
				return "https://" + e.String() + ":" + params.port, e.To4() != nil
			})...)
		}

		server.serverLogger.Info().Msgf("start HTTPS server on %s (%s)", params.effectiveAddr, strings.Join(urls, ", "))

		//start listening

		err = goServer.ListenAndServeTLS("", "")
		if err != nil {
			server.serverLogger.Print(err)
		}
	}()

	//ungracefully stop server after context is done
	go func() {
		defer func() {
			recover()
			server.endChan <- struct{}{}
		}()
		defer server.serverLogger.Info().Msg("server (" + params.effectiveAddr + ") is now closed")

		<-ctx.Done()
		server.ImmediatelyClose(ctx)
	}()

	time.Sleep(HTTP_SERVER_STARTING_WAIT_TIME)

	return server, nil
}

func (serv *HttpsServer) ListeningAddr() core.Host {
	return serv.listeningAddr
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

		eventPath := fsEvent.Path()

		for _, pathPattern := range handler.watchedPaths {
			if pathPattern.Test(nil, eventPath) {
				return true
			}
		}

		return false
	}

	serv.fsEventSource.OnIDLE(core.IdleEventSourceHandler{
		//Using core.HARD_MINIMUM_LAST_EVENT_AGE does not seem to work well (the microtask is not called).
		//Should core.HARD_MINIMUM_LAST_EVENT_AGE always be >= 2 * core.IDLE_EVENT_SOURCE_HANDLING_TICK_INTERVAL ?
		MinimumLastEventAge: 2 * core.HARD_MINIMUM_LAST_EVENT_AGE,
		IsIgnoredEvent: func(e *core.Event) bool {
			return !isRelevantEvent(e)
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

func newSymbolicHttpsServer(ctx *symbolic.Context, host *symbolic.Host, args ...symbolic.Value) (*http_ns_symb.HttpsServer, *symbolic.Error) {
	if !ctx.HasAPermissionWithKindAndType(permkind.Provide, permkind.HTTP_PERM_TYPENAME) {
		ctx.AddSymbolicGoFunctionWarning(HTTP_PROVIDE_PERM_MIGHT_BE_MISSING)
	}

	symbolic.ANY_HOST_PATTERN.PropertyNames()

	server := &http_ns_symb.HttpsServer{}

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
