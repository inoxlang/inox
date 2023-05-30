package internal

import (
	"context"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"sync"
	"time"

	"github.com/inoxlang/inox/internal/commonfmt"
	core "github.com/inoxlang/inox/internal/core"
	_dom "github.com/inoxlang/inox/internal/globals/dom"
	"github.com/inoxlang/inox/internal/permkind"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/rs/zerolog"

	fsutil "github.com/go-git/go-billy/v5/util"
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

	HANDLING_DESC_MIDDLEWARES_PROPNAME = "middlewares"
	HANDLING_DESC_ROUTING_PROPNAME     = "routing"
	HANDLING_DESC_DEFAULT_CSP_PROPNAME = "default-csp"
	HANDLING_DESC_CERTIFICATE_PROPNAME = "certificate"
	HANDLING_DESC_KEY_PROPNAME         = "key"

	HTTP_SERVER_SRC_PATH = "/http/server"
)

var (
	ErrHandlerNotSharable = errors.New("handler is not sharable")
)

// NewHttpServer returns an HttpServer with unitialized .state & .logger
func NewHttpServer(ctx *core.Context, args ...core.Value) (*HttpServer, error) {
	ctxLogger := *ctx.Logger()
	_server := &HttpServer{
		state:      ctx.GetClosestState(),
		defaultCSP: DEFAULT_CSP,
	}

	if _server.state == nil {
		return nil, errors.New("cannot create server: context's associated state is nil")
	}

	addr, _, _, userProvidedHandler, handlerValProvided, middlewares, argErr := readHttpServerArgs(ctx, _server, args...)
	if argErr != nil {
		return nil, argErr
	}

	{
		logSrc := HTTP_SERVER_SRC_PATH + "/" + addr
		_server.serverLogger = ctxLogger.With().Str(core.SOURCE_LOG_FIELD_NAME, logSrc).Logger()
		_server.securityEngine = newSecurityEngine(ctxLogger, logSrc)
	}

	var lastHandlerFn handlerFn

	if handlerValProvided {
		lastHandlerFn = createHandlerFunction(userProvidedHandler, false, _server)
	} else {
		//we set a default handler that writes "hello"
		lastHandlerFn = func(r *HttpRequest, rw *HttpResponseWriter, state *core.GlobalState, logger zerolog.Logger) {
			rw.rw.Write([]byte("hello"))
		}
	}

	middlewareFns := make([]handlerFn, len(middlewares))

	for i, val := range middlewares {
		middlewareFns[i] = createHandlerFunction(val, true, _server)
	}

	// create the http.HandlerFunc that will call lastHandlerFn & middlewares
	topHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serverLogger := _server.serverLogger

		//create the Inox values for the request and the response writer
		req, err := NewServerSideRequest(r, serverLogger, _server)
		if err != nil {
			serverLogger.Print(err)
			w.WriteHeader(http.StatusBadRequest)
		}

		rw := NewResponseWriter(req, w, serverLogger)

		// rate limiting & more

		if _server.securityEngine.rateLimitRequest(req, rw) {
			rw.writeStatus(http.StatusTooManyRequests)
			return
		}

		//create a global state for handling the request
		handlerCtx := ctx.BoundChild()
		defer handlerCtx.CancelIfShortLived()

		if !req.AcceptAny() && !req.ParsedAcceptHeader.Match(core.EVENT_STREAM_CTYPE) {
			core.StartNewTransaction(handlerCtx, core.Option{
				Name:  core.TX_TIMEOUT_OPTION_NAME,
				Value: core.Duration(DEFAULT_HTTP_SERVER_TX_TIMEOUT),
			})
		}

		//transaction is cleaned up during context cancelation, so no need to defer a rollback

		handlerGlobalState := core.NewGlobalState(handlerCtx)
		handlerGlobalState.Logger = _server.state.Logger
		handlerGlobalState.Out = _server.state.Out
		handlerGlobalState.Module = _server.state.Module
		handlerGlobalState.SystemGraph = _server.state.SystemGraph

		//

		if req.NewSession {
			addSessionIdCookie(rw, req.Session.Id)
		}

		defer rw.FinalLog()

		//call middlewares & handler

		for _, fn := range middlewareFns {
			fn(req, rw, handlerGlobalState, serverLogger)
			if rw.finished {
				return
			}
		}

		lastHandlerFn(req, rw, handlerGlobalState, serverLogger)
	})

	//create a stdlib http Server
	server, err := NewGolangHttpServer(addr, topHandler, "", "", ctx)
	if err != nil {
		return nil, err
	}

	endChan := make(chan struct{}, 1)

	_server.wrappedServer = server
	_server.endChan = endChan

	//listen and serve in a goroutine
	go func() {
		_server.serverLogger.Info().Msg("serve " + addr)

		err := server.ListenAndServeTLS("", "")
		if err != nil {
			_server.serverLogger.Print(err)
		}
		endChan <- struct{}{}
	}()

	//ungracefully stop server after context is done
	go func() {
		<-ctx.Done()
		_server.ImmediatelyClose(ctx)
	}()

	time.Sleep(HTTP_SERVER_STARTING_WAIT_TIME)

	return _server, nil
}

func readHttpServerArgs(ctx *core.Context, server *HttpServer, args ...core.Value) (
	addr string,
	certificate string,
	certKey *core.Secret,
	userProvidedHandler core.Value,
	handlerValProvided bool,
	middlewares []core.Value,
	argErr error,
) {

	const HANDLING_ARG_NAME = "handler/handling"

	for _, arg := range args {
		switch v := arg.(type) {
		case core.Host:
			if addr != "" {
				argErr = errors.New("address already provided")
				return
			}
			parsed, _ := url.Parse(string(v))
			if v.Scheme() != "https" {
				argErr = fmt.Errorf("invalid scheme '%s'", v)
				return
			}
			server.host = v
			addr = parsed.Host

			perm := core.HttpPermission{Kind_: permkind.Provide, Entity: v}
			if err := ctx.CheckHasPermission(perm); err != nil {
				argErr = err
				return
			}
		case *core.InoxFunction:
			if handlerValProvided {
				argErr = core.FmtErrArgumentProvidedAtLeastTwice(HANDLING_ARG_NAME)
				return
			}

			if ok, expl := v.IsSharable(server.state); !ok {
				argErr = fmt.Errorf("%w: %s", ErrHandlerNotSharable, expl)
				return
			}
			v.Share(server.state)
			userProvidedHandler = v
			handlerValProvided = true
		case *core.GoFunction:
			if handlerValProvided {
				argErr = core.FmtErrArgumentProvidedAtLeastTwice(HANDLING_ARG_NAME)
				return
			}
			if ok, expl := v.IsSharable(server.state); !ok {
				argErr = fmt.Errorf("%w: %s", ErrHandlerNotSharable, expl)
				return
			}
			v.Share(server.state)
			userProvidedHandler = v
			handlerValProvided = true
		case *core.Mapping:
			if handlerValProvided {
				argErr = core.FmtErrArgumentProvidedAtLeastTwice(HANDLING_ARG_NAME)
				return
			}
			if ok, expl := v.IsSharable(server.state); !ok {
				argErr = fmt.Errorf("%w: %s", ErrHandlerNotSharable, expl)
			}
			v.Share(server.state)

			userProvidedHandler = v
			handlerValProvided = true
		case *core.Object:
			if handlerValProvided {
				argErr = core.FmtErrArgumentProvidedAtLeastTwice(HANDLING_ARG_NAME)
				return
			}
			handlerValProvided = true

			// extract routing handler, middlewares, ... from description
			for propKey, propVal := range v.EntryMap() {
				switch propKey {
				case HANDLING_DESC_MIDDLEWARES_PROPNAME:
					iterable, ok := propVal.(core.Iterable)
					if !ok {
						argErr = core.FmtPropOfArgXShouldBeOfTypeY(propKey, HANDLING_ARG_NAME, "iterable", propVal)
						return
					}

					it := iterable.Iterator(ctx, core.IteratorConfiguration{})
					for it.Next(ctx) {
						e := it.Value(ctx)
						if !isValidHandlerValue(e) {
							s := fmt.Sprintf("%s is not a middleware", core.Stringify(e, ctx))
							argErr = core.FmtUnexpectedElementInPropIterableOfArgX(propKey, HANDLING_ARG_NAME, s)
							return
						}

						if psharable, ok := e.(core.PotentiallySharable); ok && utils.Ret0(psharable.IsSharable(server.state)) {
							psharable.Share(server.state)
						} else {
							s := fmt.Sprintf("%s is not sharable", core.Stringify(e, ctx))
							argErr = core.FmtUnexpectedElementInPropIterableOfArgX(propKey, HANDLING_ARG_NAME, s)
							return
						}
						middlewares = append(middlewares, e)
					}
				case HANDLING_DESC_ROUTING_PROPNAME:
					if !isValidHandlerValue(propVal) {
						argErr = core.FmtUnexpectedValueAtKeyofArgShowVal(propVal, propKey, HANDLING_ARG_NAME)
					}

					if psharable, ok := propVal.(core.PotentiallySharable); ok && utils.Ret0(psharable.IsSharable(server.state)) {
						psharable.Share(server.state)
					} else {
						argErr = core.FmtPropOfArgXShouldBeY(propKey, HANDLING_ARG_NAME, "sharable")
						return
					}

					userProvidedHandler = propVal
				case HANDLING_DESC_DEFAULT_CSP_PROPNAME:
					csp, ok := propVal.(*_dom.ContentSecurityPolicy)
					if !ok {
						argErr = core.FmtUnexpectedValueAtKeyofArgShowVal(propVal, propKey, HANDLING_ARG_NAME)
						return
					}
					server.defaultCSP = csp
				case HANDLING_DESC_CERTIFICATE_PROPNAME:
					certVal, ok := propVal.(core.StringLike)
					if !ok {
						argErr = core.FmtUnexpectedValueAtKeyofArgShowVal(propVal, propKey, HANDLING_ARG_NAME)
						return
					}
					certificate = certVal.GetOrBuildString()
				case HANDLING_DESC_KEY_PROPNAME:
					secret, ok := propVal.(*core.Secret)
					if !ok {
						argErr = core.FmtUnexpectedValueAtKeyofArgShowVal(propVal, propKey, HANDLING_ARG_NAME)
						return
					}
					certKey = secret
				default:
					argErr = commonfmt.FmtUnexpectedPropInArgX(propKey, HANDLING_ARG_NAME)
				}
			}

			if userProvidedHandler == nil {
				argErr = core.FmtMissingPropInArgX(HANDLING_DESC_ROUTING_PROPNAME, HANDLING_ARG_NAME)
			}
		default:
			argErr = fmt.Errorf("http.server: invalid argument of type %T", v)
		}
	}

	if addr == "" {
		argErr = errors.New("no address provided")
		return
	}

	return
}

func NewGolangHttpServer(addr string, handler http.Handler, pemEncodedCert string, pemEncodedKey string, ctx *core.Context) (*http.Server, error) {
	fls := ctx.GetFileSystem()

	if pemEncodedCert == "" { //if no certificate provided by the user we create one
		//we generate a self signed certificate that we write to disk so that
		//we can reuse it
		CERT_FILEPATH := "localhost.cert"
		CERT_KEY_FILEPATH := "localhost.key"

		_, err1 := fls.Stat(CERT_FILEPATH)
		_, err2 := fls.Stat(CERT_KEY_FILEPATH)

		if errors.Is(err1, os.ErrNotExist) || errors.Is(err2, os.ErrNotExist) {

			if err1 == nil {
				fls.Remove(CERT_FILEPATH)
			}

			if err2 == nil {
				fls.Remove(CERT_KEY_FILEPATH)
			}

			cert, key, err := generateSelfSignedCertAndKey()
			if err != nil {
				return nil, err
			}

			certFile, err := fls.Create(CERT_FILEPATH)
			if err != nil {
				return nil, err
			}
			pem.Encode(certFile, cert)
			pemEncodedCert = string(pem.EncodeToMemory(cert))

			certFile.Close()
			keyFile, err := fls.Create(CERT_KEY_FILEPATH)
			if err != nil {
				return nil, err
			}
			pem.Encode(keyFile, key)
			keyFile.Close()
			pemEncodedKey = string(pem.EncodeToMemory(key))
		} else if err1 == nil && err2 == nil {
			certFile, err := fsutil.ReadFile(fls, CERT_FILEPATH)
			if err != nil {
				return nil, err
			}
			keyFile, err := fsutil.ReadFile(fls, CERT_KEY_FILEPATH)
			if err != nil {
				return nil, err
			}

			pemEncodedCert = string(certFile)
			pemEncodedKey = string(keyFile)
		} else {
			return nil, fmt.Errorf("%w %w", err1, err2)
		}
	}

	tlsConfig, err := GetTLSConfig(ctx, pemEncodedCert, pemEncodedKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get TLS config: %w", err)
	}

	server := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: DEFAULT_HTTP_SERVER_READ_HEADER_TIMEOUT,
		ReadTimeout:       DEFAULT_HTTP_SERVER_READ_TIMEOUT,
		WriteTimeout:      DEFAULT_HTTP_SERVER_WRITE_TIMEOUT,
		MaxHeaderBytes:    DEFAULT_HTTP_SERVER_MAX_HEADER_BYTES,
		TLSConfig:         tlsConfig,
		//TODO: set logger
	}

	return server, nil
}

// HttpServer implements the GoValue interface.
type HttpServer struct {
	core.NoReprMixin
	core.NotClonableMixin

	host           core.Host
	wrappedServer  *http.Server
	lock           sync.RWMutex
	endChan        chan struct{}
	state          *core.GlobalState
	defaultCSP     *_dom.ContentSecurityPolicy
	securityEngine *securityEngine
	serverLogger   zerolog.Logger

	sseServer *SseServer
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
	return []string{"wait_closed", "close"}
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
