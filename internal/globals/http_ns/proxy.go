package http_ns

import (
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"runtime/debug"
	"strconv"

	"github.com/elazarl/goproxy"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/permkind"
	"github.com/inoxlang/inox/internal/mimeconsts"
	netaddr "github.com/inoxlang/inox/internal/netaddr"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/rs/zerolog"
)

type HTTPProxyParams struct {
	Port int

	//OnRequest is invoked before GetContext and before any permissions is checked to forward the request.
	//If a non nil response is returned it it sent to the client.
	OnRequest func(req *http.Request, proxyCtx *goproxy.ProxyCtx) (*http.Request, *http.Response)

	//GetContext is called to retrieve the context for the request, the context is used to check permissions.
	GetContext func(req *http.Request) *core.Context

	RemovedRequestHeaders []string

	Logger zerolog.Logger
}

// StartHTTPProxy starts an HTTP proxy listening on 127.0.0.1:<port>.
// The connection between the client and an HTTP proxy is not encrypted,
// https://chromium.googlesource.com/chromium/src/+/HEAD/net/docs/proxy.md#http-proxy-scheme.
func MakeHTTPProxy(ctx *core.Context, params HTTPProxyParams) (*http.Server, error) {

	addr := "127.0.0.1:" + strconv.Itoa(params.Port)

	perm := core.HttpPermission{Kind_: permkind.Provide, Entity: core.Host("http://" + addr)}

	if err := ctx.CheckHasPermission(perm); err != nil {
		return nil, err
	}

	proxy := goproxy.NewProxyHttpServer()
	proxy.Verbose = true
	proxy.Logger = printfLogger{params.Logger}

	proxy.NonproxyHandler = http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Host == "" {
			fmt.Fprintln(w, "Cannot handle requests without Host header, e.g., HTTP 1.0")
			return
		}
		req.URL.Scheme = "http"
		req.URL.Host = req.Host
		proxy.ServeHTTP(w, req)
	})

	proxy.OnRequest(goproxy.ReqHostMatches(regexp.MustCompile("^.*$"))).
		HandleConnect(goproxy.AlwaysMitm)

	proxy.OnRequest().DoFunc(func(req *http.Request, proxyCtx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
		defer func() {
			e := recover()
			if e != nil {
				err := fmt.Errorf("%w: %s", utils.ConvertPanicValueToError(e), debug.Stack())
				params.Logger.Debug().Err(err).Send()
			}
		}()

		//check the request was sent from localhost.
		addr, err := netaddr.RemoteAddrWithPortFrom(req.RemoteAddr)
		if err != nil {
			body := fmt.Sprintf("failed to get the get the IP:port of the sender: %s\n", err.Error())
			resp := goproxy.NewResponse(req, mimeconsts.HTML_CTYPE, http.StatusInternalServerError, body)
			resp.Status = "proxy error, invalid ip:port"
			return req, resp
		}
		if !isLocalhostOr127001Addr(addr) {
			body := fmt.Sprintf("the sender has an invalid IP:port : %s, only requests from localhost are allowed\n", addr)
			resp := goproxy.NewResponse(req, mimeconsts.HTML_CTYPE, http.StatusInternalServerError, body)
			resp.Status = "proxy error, invalid ip:port"
			return req, resp
		}

		req, resp := params.OnRequest(req, proxyCtx)
		if resp != nil {
			return req, resp
		}

		//get the context
		ctx := params.GetContext(req)

		for _, name := range params.RemovedRequestHeaders {
			delete(req.Header, name)
		}

		if ctx == nil {
			params.Logger.Debug().Err(errors.New("failed to get context for proxied request")).Send()
			resp := goproxy.NewResponse(req, mimeconsts.HTML_CTYPE, http.StatusInternalServerError, "")
			resp.Status = "proxy error"
			return req, resp
		}

		//check permissions and remove a token

		u := *req.URL
		//remove port if https:443 or http:80
		if (req.URL.Port() == "443" && req.URL.Scheme == "https") ||
			(req.URL.Port() == "80" && req.URL.Scheme == "http") {
			u.Host = u.Hostname()
		}

		perm, err := getPermForRequest(req.Method, core.URL(u.String()))
		if err != nil {
			params.Logger.Err(err).Send()
			ctx.Logger().Err(err).Send()
			return nil, nil
		}

		if err := ctx.CheckHasPermission(perm); err != nil {
			params.Logger.Err(err).Send()
			ctx.Logger().Err(err).Send()

			resp := goproxy.NewResponse(req, mimeconsts.PLAIN_TEXT_CTYPE, http.StatusInternalServerError, "")
			resp.Status = err.Error()
			return req, resp
		}

		ctx.Take(HTTP_REQUEST_RATE_LIMIT_NAME, 1)

		return req, nil
	})

	proxy.OnResponse().DoFunc(func(resp *http.Response, ctx *goproxy.ProxyCtx) *http.Response {
		return resp
	})

	return &http.Server{Addr: addr, Handler: proxy}, nil
}

type printfLogger struct {
	zerolog.Logger
}

func (l printfLogger) Printf(format string, v ...interface{}) {
	l.Debug().Msgf(format, v...)
}
