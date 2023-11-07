package chrome_ns

import (
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/elazarl/goproxy"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/http_ns"
	"github.com/rs/zerolog"
)

const (
	BROWSER_PROXY_PORT = 12750
	BROWSER_PROXY_ADDR = "127.0.0.1:12750"

	//After a browser instance is created it navigates to this URL.
	//The request is intercepted by the proxy, no server is listening on this port.
	//The hostname should not be changed because using the loopback allows to check that
	//requests to the loopback do not bypass the proxy.
	CHROME_INSTANCE_REGISTRATION_URL_PREFIX = "https://127.0.0.1:9999/register-browser-instance/"
)

var (
	proxyStarted atomic.Bool

	handleIdToContext     = map[string]*core.Context{}
	handleIdToContextLock sync.Mutex
)

// StartSharedProxy starts an HTTP proxy in another goroutine, the proxy is used by all browser instances
// controlled by the current package.
func StartSharedProxy() {
	//https://chromium.googlesource.com/chromium/src/+/HEAD/net/docs/proxy.md#HTTP-proxy-scheme

	if !proxyStarted.CompareAndSwap(false, true) {
		return
	}

	go http_ns.StartHTTPProxy(http_ns.HTTPProxyParams{
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
		Logger:                zerolog.New(os.Stdout),
		RemovedRequestHeaders: []string{HANDLE_ID_HEADER},
	})

	time.Sleep(10 * time.Millisecond)
}
