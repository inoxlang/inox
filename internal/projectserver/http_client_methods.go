package projectserver

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"runtime/debug"
	"time"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/http_ns"
	"github.com/inoxlang/inox/internal/inoxconsts"
	"github.com/inoxlang/inox/internal/projectserver/jsonrpc"
	"github.com/inoxlang/inox/internal/projectserver/lsp"
	"github.com/inoxlang/inox/internal/utils"
)

const (
	//make the project server sends a request, the response is sent asynchronously to the LSP client with a notification.
	HTTP_REQUEST_ASYNC_METHOD              = "httpClient/requestAsync"
	HTTP_RESPONSE_EVENT_METHOD             = "httpClient/responseEvent"
	DEFAULT_HTTP_CLIENT_TIMEOUT            = 20 * time.Second
	DEFAULT_HTTP_RESPONSE_BODY_LIMIT int64 = 5_000_000
)

var (
	//Used to prevent redirection, the direction should be made by the HTTP client on the developer's side.
	errNoRedirect = errors.New("no redirect")
)

type HttpRequestParams struct {
	RequestID  string              `json:"reqID"`
	URL        string              `json:"url"`
	Method     string              `json:"method"`
	Headers    map[string][]string `json:"headers"` //we should not use a http.Header here because http.Header's keys are normalized
	BodyBase64 string              `json:"body"`
}

type HttpResponseEvent struct {
	RequestID  string      `json:"reqID"`
	Error      string      `json:"error,omitempty"`
	StatusCode int         `json:"statusCode,omitempty"` //not set if error
	Headers    http.Header `json:"headers,omitempty"`    //not set if error
	BodyBase64 string      `json:"body,omitempty"`       //not set if error
}

func registerHttpClientMethods(server *lsp.Server, opts LSPServerConfiguration) {
	server.OnCustom(jsonrpc.MethodInfo{
		Name:         HTTP_REQUEST_ASYNC_METHOD,
		RateLimits:   []int{10, 50, 500},
		AvoidLogging: true,
		NewRequest: func() interface{} {
			return &HttpRequestParams{}
		},
		Handler: handleHttpRequest,
	})
}

func handleHttpRequest(callCtx context.Context, req interface{}) (interface{}, error) {
	rpcSession := jsonrpc.GetSession(callCtx)
	params := req.(*HttpRequestParams)
	ctx := rpcSession.Context()

	//-----------------------------------------------
	session := getCreateLockedProjectSession(rpcSession)
	var client *http.Client

	func() {
		defer session.lock.Unlock()

		if session.secureHttpClient == nil {
			session.secureHttpClient = &http.Client{
				CheckRedirect: ignoreRedirects,
				Timeout:       DEFAULT_HTTP_CLIENT_TIMEOUT,
			}

		}
		if session.insecureHttpClient == nil {
			session.insecureHttpClient = &http.Client{
				CheckRedirect: ignoreRedirects,
				Timeout:       DEFAULT_HTTP_CLIENT_TIMEOUT,
				Transport: &http.Transport{
					TLSClientConfig: &tls.Config{
						InsecureSkipVerify: true,
					},
				},
			}
		}
	}()
	//-----------------------------------------------

	//Create a HTTP request from the information and body present in parameters.

	u, err := url.Parse(params.URL)
	if err != nil {
		return nil, jsonrpc.ResponseError{
			Code:    jsonrpc.InternalError.Code,
			Message: fmt.Sprintf("invalid URL: %s", err.Error()),
		}
	}
	if u.Hostname() == "localhost" {
		client = session.insecureHttpClient
	} else { //for now only sending requests to localhost is allowed.
		event := HttpResponseEvent{
			RequestID: params.RequestID,
			Error:     "only sending requests to localhost is allowed",
		}

		notifParams, err := json.Marshal(event)
		if err != nil {
			return nil, nil
		}

		go func() {
			defer utils.Recover()

			rpcSession.Notify(jsonrpc.NotificationMessage{
				Method: HTTP_RESPONSE_EVENT_METHOD,
				Params: json.RawMessage(notifParams),
			})
		}()

		//client = data.secureHttpClient
		return nil, nil
	}

	var body io.Reader
	if params.BodyBase64 != "" {
		p, err := base64.StdEncoding.DecodeString(params.BodyBase64)
		if err != nil {
			if err != nil {
				return nil, jsonrpc.ResponseError{
					Code:    jsonrpc.InternalError.Code,
					Message: fmt.Sprintf("failed to decode body of HTTP request: %s", err.Error()),
				}
			}
		}

		body = bytes.NewReader(p)
	}

	httpReq, err := http.NewRequest(params.Method, params.URL, body)
	if err != nil {
		return nil, jsonrpc.ResponseError{
			Code:    jsonrpc.InternalError.Code,
			Message: fmt.Sprintf("failed to create HTTP request: %s", err.Error()),
		}
	}

	for headerName, values := range params.Headers {
		for _, val := range values {
			httpReq.Header.Add(headerName, val)
		}
	}

	httpReq.Header.Del("host")
	httpReq.Header.Del("connection")

	devSessionKey, ok := ctx.ResolveUserData(http_ns.CTX_DATA_KEY_FOR_DEV_SESSION_KEY).(core.String)
	if ok {
		httpReq.Header.Set(inoxconsts.DEV_SESSION_KEY_HEADER, string(devSessionKey))
	}

	//Create a goroutine that will send the request and notify the response.
	//The execution of the goroutine is not cancellable by $callCtx because
	//the handler returns early.

	go func() {
		defer func() {
			e := recover()
			if e != nil {
				err := utils.ConvertPanicValueToError(e)
				err = fmt.Errorf("%w: %s", err, debug.Stack())
				rpcSession.Logger().Println("HTTP Request", "(id "+params.RequestID+")", err)
			}
		}()

		ctx := rpcSession.Context().BoundChild()
		defer ctx.CancelGracefully()

		result := HttpResponseEvent{RequestID: params.RequestID}

		httpReq = httpReq.WithContext(ctx)
		resp, err := client.Do(httpReq)

		if err != nil && err.(*url.Error).Err.Error() != errNoRedirect.Error() {
			result.Error = err.Error()
		} else {
			result.StatusCode = resp.StatusCode
			result.Headers = resp.Header

			//Read the body.

			limit := DEFAULT_HTTP_RESPONSE_BODY_LIMIT

			reader := io.LimitReader(resp.Body, limit)
			body, err := io.ReadAll(reader)

			event := HttpResponseEvent{RequestID: params.RequestID}

			if err != nil {
				event.Error = err.Error()
				event.BodyBase64 = ""
				event.StatusCode = 0
			} else {
				result.BodyBase64 = base64.StdEncoding.EncodeToString(body)
			}

		}

		//Marshal the parameters and send the notification.

		notifParams, err := json.Marshal(result)
		if err != nil {
			return
		}

		rpcSession.Notify(jsonrpc.NotificationMessage{
			Method: HTTP_RESPONSE_EVENT_METHOD,
			Params: json.RawMessage(notifParams),
		})
	}()

	//Acknowledge reception

	return nil, nil
}

// ignoreRedirects always returns an error so that the client on the project server's side does not follow the redirection.
func ignoreRedirects(req *http.Request, via []*http.Request) error {
	return errNoRedirect
}
