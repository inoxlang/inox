package chrome_ns

import (
	"errors"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/elazarl/goproxy"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/http_ns"
	"github.com/inoxlang/inox/internal/utils"
)

const (
	BROWSER_PROXY_PORT = 12750
	BROWSER_PROXY_ADDR = "127.0.0.1:12750"

	//After a browser instance is created it navigates to this URL.
	//The request is intercepted by the proxy, no server is listening on this port.
	//The hostname should not be changed because using the loopback allows to check that
	//requests to the loopback do not bypass the proxy.
	CHROME_INSTANCE_REGISTRATION_URL_PREFIX = "https://127.0.0.1:9999/register-browser-instance/"

	BROWSER_PROXY_SRC_NAME = "/browser-proxy"
)

var (
	proxyStarted atomic.Bool

	handleIdToContext     = map[string]*core.Context{}
	handleIdToContextLock sync.Mutex

	ErrProxyAlreadyStarted = errors.New("browser proxy already started")
)

// StartSharedProxy starts an HTTP proxy in another goroutine, the proxy is used by all browser instances
// controlled by the current package.
func StartSharedProxy(ctx *core.Context) error {
	if !browserAutomationAllowed.Load() {
		return ErrBrowserAutomationNotAllowed
	}

	//https://chromium.googlesource.com/chromium/src/+/HEAD/net/docs/proxy.md#HTTP-proxy-scheme

	if !proxyStarted.CompareAndSwap(false, true) {
		return nil
	}
	logger := ctx.Logger().With().Str(core.SOURCE_LOG_FIELD_NAME, BROWSER_PROXY_SRC_NAME).Logger()

	proxyServer, err := http_ns.MakeHTTPProxy(ctx, http_ns.HTTPProxyParams{
		Port: BROWSER_PROXY_PORT,
		OnRequest: func(req *http.Request, proxyCtx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
			return req, nil
		},
		GetContext: func(req *http.Request) *core.Context {
			values := req.Header[HANDLE_ID_HEADER]
			if len(values) == 0 {
				return nil
			}
			handleIdToContextLock.Lock()
			defer handleIdToContextLock.Unlock()

			return handleIdToContext[values[0]]
		},
		Logger:                logger,
		RemovedRequestHeaders: []string{HANDLE_ID_HEADER},
	})

	if err != nil {
		return err
	}

	go func() {
		defer utils.Recover()
		logger.Debug().Msgf("start browser proxy server listening on %s", BROWSER_PROXY_ADDR)
		proxyServer.ListenAndServe()
	}()

	time.Sleep(10 * time.Millisecond)
	return nil
}
