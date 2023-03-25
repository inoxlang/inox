package internal

import (
	"context"
	"encoding/pem"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"sync"
	"time"

	"github.com/inox-project/inox/internal/commonfmt"
	core "github.com/inox-project/inox/internal/core"
	_dom "github.com/inox-project/inox/internal/globals/dom"

	"github.com/inox-project/inox/internal/utils"
)

const (
	DEFAULT_HTTP_SERVER_READ_TIMEOUT     = 8 * time.Second
	DEFAULT_HTTP_SERVER_WRITE_TIMEOUT    = 12 * time.Second
	DEFAULT_HTTP_SERVER_MAX_HEADER_BYTES = 1 << 12
	DEFAULT_HTTP_SERVER_TX_TIMEOUT       = 20 * time.Second
	SSE_STREAM_WRITE_TIMEOUT             = 500 * time.Second

	HTTP_SERVER_STARTING_WAIT_TIME        = 5 * time.Millisecond
	HTTP_SERVER_GRACEFUL_SHUTDOWN_TIMEOUT = 5 * time.Second

	HANDLING_DESC_MIDDLEWARES_KEY = "middlewares"
	HANDLING_DESC_ROUTING_KEY     = "routing"
	HANDLING_DESC_DEFAULT_CSP_KEY = "default-csp"
)

// NewHttpServer returns an HttpServer with unitialized .state & .logger
func NewHttpServer(ctx *core.Context, args ...core.Value) (*HttpServer, error) {
	var (
		addr                string
		userProvidedHandler core.Value
		handlerValProvided  bool
		middlewares         []core.Value

		_server = &HttpServer{
			state:      ctx.GetClosestState(),
			defaultCSP: DEFAULT_CSP,
		}
	)

	if _server.state == nil {
		return nil, errors.New("cannot create server: context's associated state is nil")
	}

	const HANDLING_ARG_NAME = "handler/handling"

	for _, arg := range args {
		switch v := arg.(type) {
		case core.Host:
			if addr != "" {
				return nil, errors.New("address already provided")
			}
			parsed, _ := url.Parse(string(v))
			if v.Scheme() != "https" {
				return nil, fmt.Errorf("invalid scheme '%s'", v)
			}
			_server.host = v
			addr = parsed.Host

			perm := core.HttpPermission{Kind_: core.ProvidePerm, Entity: v}
			if err := ctx.CheckHasPermission(perm); err != nil {
				return nil, err
			}
		case *core.InoxFunction:
			if handlerValProvided {
				return nil, core.FmtErrArgumentProvidedAtLeastTwice(HANDLING_ARG_NAME)
			}

			if !v.IsSharable(_server.state) {
				return nil, errors.New("handler is not sharable")
			}
			v.Share(_server.state)
			userProvidedHandler = v
			handlerValProvided = true
		case *core.GoFunction:
			if handlerValProvided {
				return nil, core.FmtErrArgumentProvidedAtLeastTwice(HANDLING_ARG_NAME)
			}
			if !v.IsSharable(_server.state) {
				return nil, errors.New("handler is not sharable")
			}
			v.Share(_server.state)
			userProvidedHandler = v
			handlerValProvided = true
		case *core.Mapping:
			if handlerValProvided {
				return nil, core.FmtErrArgumentProvidedAtLeastTwice(HANDLING_ARG_NAME)
			}
			if !v.IsSharable(_server.state) {
				return nil, errors.New("handler is not sharable")
			}
			v.Share(_server.state)

			userProvidedHandler = v
			handlerValProvided = true
		case *core.Object:
			if handlerValProvided {
				return nil, core.FmtErrArgumentProvidedAtLeastTwice(HANDLING_ARG_NAME)
			}
			handlerValProvided = true

			// extract routing handler, middlewares, ... from description
			for propKey, propVal := range v.EntryMap() {
				switch propKey {
				case HANDLING_DESC_MIDDLEWARES_KEY:
					iterable, ok := propVal.(core.Iterable)
					if !ok {
						return nil, core.FmtPropOfArgXShouldBeOfTypeY(HANDLING_DESC_MIDDLEWARES_KEY, HANDLING_ARG_NAME, "iterable", propVal)
					}

					it := iterable.Iterator(ctx, core.IteratorConfiguration{})
					for it.Next(ctx) {
						e := it.Value(ctx)
						if !isValidHandlerValue(e) {
							s := fmt.Sprintf("%s is not a middleware", core.Stringify(e, ctx))
							return nil, core.FmtUnexpectedElementInPropIterableOfArgX(HANDLING_DESC_MIDDLEWARES_KEY, HANDLING_ARG_NAME, s)
						}

						if psharable, ok := e.(core.PotentiallySharable); ok && psharable.IsSharable(_server.state) {
							psharable.Share(_server.state)
						} else {
							s := fmt.Sprintf("%s is not sharable", core.Stringify(e, ctx))
							return nil, core.FmtUnexpectedElementInPropIterableOfArgX(HANDLING_DESC_MIDDLEWARES_KEY, HANDLING_ARG_NAME, s)
						}
						middlewares = append(middlewares, e)
					}
				case HANDLING_DESC_ROUTING_KEY:
					if !isValidHandlerValue(propVal) {
						return nil, core.FmtUnexpectedValueAtKeyofArgShowVal(propVal, HANDLING_DESC_ROUTING_KEY, HANDLING_ARG_NAME)
					}

					if psharable, ok := propVal.(core.PotentiallySharable); ok && psharable.IsSharable(_server.state) {
						psharable.Share(_server.state)
					} else {
						return nil, core.FmtPropOfArgXShouldBeY(HANDLING_DESC_ROUTING_KEY, HANDLING_ARG_NAME, "sharable")
					}

					userProvidedHandler = propVal
				case HANDLING_DESC_DEFAULT_CSP_KEY:
					csp, ok := propVal.(*_dom.ContentSecurityPolicy)
					if !ok {
						return nil, core.FmtUnexpectedValueAtKeyofArgShowVal(propVal, HANDLING_DESC_DEFAULT_CSP_KEY, HANDLING_ARG_NAME)
					}
					_server.defaultCSP = csp
				default:
					return nil, commonfmt.FmtUnexpectedPropInArgX(propKey, HANDLING_ARG_NAME)
				}
			}

			if userProvidedHandler == nil {
				return nil, core.FmtMissingPropInArgX(HANDLING_DESC_ROUTING_KEY, HANDLING_ARG_NAME)
			}
		default:
			return nil, fmt.Errorf("http.server: invalid argument of type %T", v)
		}
	}

	if addr == "" {
		return nil, errors.New("no address provided")
	}

	var lastHandlerFn handlerFn

	if handlerValProvided {
		lastHandlerFn = createHandlerFunction(userProvidedHandler, false, _server)
	} else {
		//we set a default handler that writes "hello"
		lastHandlerFn = func(r *HttpRequest, rw *HttpResponseWriter, state *core.GlobalState, logger *log.Logger) {
			rw.rw.Write([]byte("hello"))
		}
	}

	middlewareFns := make([]handlerFn, len(middlewares))

	for i, val := range middlewares {
		middlewareFns[i] = createHandlerFunction(val, true, _server)
	}

	// create the http.HandlerFunc that will call lastHandlerFn & middlewares
	topHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		logger := _server.state.Logger

		if logger == nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		//create the Inox values for the request and the response writer
		req, err := NewServerSideRequest(r, logger, _server)
		if err != nil {
			logger.Println(err)
			w.WriteHeader(http.StatusBadRequest)
		}

		rw := NewResponseWriter(req, w)

		logger.Println(utils.AddCarriageReturnAfterNewlines(fmt.Sprintf("%s %s", req.Method, req.Path)))

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

		//

		if req.NewSession {
			addSessionIdCookie(rw, req.Session.Id)
		}

		//call handler

		defer func() {
			s := fmt.Sprintf("%s %s handled (%dms): %d %s",
				req.Method, req.Path, time.Since(start).Milliseconds(), rw.Status(), http.StatusText(rw.Status()))
			logger.Println(utils.AddCarriageReturnAfterNewlines(s))
		}()

		for _, fn := range middlewareFns {
			fn(req, rw, handlerGlobalState, logger)
			if rw.finished {
				return
			}
		}

		lastHandlerFn(req, rw, handlerGlobalState, logger)
	})

	server, certFile, keyFile, err := makeHttpServer(addr, topHandler, "", "")
	if err != nil {
		return nil, err
	}

	endChan := make(chan struct{}, 1)

	_server.wrappedServer = server
	_server.endChan = endChan

	//listen and serve in a goroutine
	go func() {
		logger := _server.state.Logger
		if logger != nil {
			logger.Println("serve", addr)
		}

		err := server.ListenAndServeTLS(certFile, keyFile)
		if logger != nil && err != nil {
			logger.Println(err)
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

// NewFileServer returns an HttpServer that uses Go's http.FileServer(dir) to handle requests
func NewFileServer(ctx *core.Context, args ...core.Value) (*HttpServer, error) {
	var addr string
	var dir core.Path

	for _, arg := range args {
		switch v := arg.(type) {
		case core.Host:
			if addr != "" {
				return nil, errors.New("address already provided")
			}
			parsed, _ := url.Parse(string(v))
			addr = parsed.Host

			perm := core.HttpPermission{Kind_: core.ProvidePerm, Entity: v}
			if err := ctx.CheckHasPermission(perm); err != nil {
				return nil, err
			}
		case core.Path:
			if !v.IsDirPath() {
				return nil, errors.New("the directory path should end with '/'")
			}
			dir = v
		default:
		}
	}

	if addr == "" {
		return nil, errors.New("no address provided")
	}

	if dir == "" {
		return nil, errors.New("no (directory) path required")
	}

	server, certFile, keyFile, err := makeHttpServer(addr, http.FileServer(http.Dir(dir)), "", "")
	if err != nil {
		return nil, err
	}

	endChan := make(chan struct{}, 1)

	go func() {
		log.Println(server.ListenAndServeTLS(certFile, keyFile))
		endChan <- struct{}{}
	}()

	time.Sleep(5 * time.Millisecond)
	return &HttpServer{
		wrappedServer: server,
		endChan:       endChan,
	}, nil
}

func makeHttpServer(addr string, handler http.Handler, certFilePath string, keyFilePath string) (*http.Server, string, string, error) {

	//we generate a self signed certificate that we write to disk so that
	//we can reuse it
	CERT_FILEPATH := "localhost.cert"
	CERT_KEY_FILEPATH := "localhost.key"

	_, err1 := os.Stat(CERT_FILEPATH)
	_, err2 := os.Stat(CERT_KEY_FILEPATH)

	if errors.Is(err1, os.ErrNotExist) || errors.Is(err2, os.ErrNotExist) {

		if err1 == nil {
			os.Remove(CERT_FILEPATH)
		}

		if err2 == nil {
			os.Remove(CERT_KEY_FILEPATH)
		}

		cert, key, err := generateSelfSignedCertAndKey()
		if err != nil {
			return nil, "", "", err
		}

		certFile, err := os.Create(CERT_FILEPATH)
		if err != nil {
			return nil, "", "", err
		}
		pem.Encode(certFile, cert)
		certFile.Close()

		keyFile, err := os.Create(CERT_KEY_FILEPATH)
		if err != nil {
			return nil, "", "", err
		}
		pem.Encode(keyFile, key)
		keyFile.Close()
	}

	server := &http.Server{
		Addr:           addr,
		Handler:        handler,
		ReadTimeout:    DEFAULT_HTTP_SERVER_READ_TIMEOUT,
		WriteTimeout:   DEFAULT_HTTP_SERVER_WRITE_TIMEOUT,
		MaxHeaderBytes: DEFAULT_HTTP_SERVER_MAX_HEADER_BYTES,
	}

	return server, CERT_FILEPATH, CERT_KEY_FILEPATH, nil
}

// HttpServer implements the GoValue interface.
type HttpServer struct {
	core.NoReprMixin
	core.NotClonableMixin

	host          core.Host
	wrappedServer *http.Server
	lock          sync.RWMutex
	endChan       chan struct{}
	state         *core.GlobalState
	defaultCSP    *_dom.ContentSecurityPolicy

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
	case "waitClosed":
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
	return []string{"waitClosed", "close"}
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
