package internal

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"

	"github.com/aohorodnyk/mimeheader"
	core "github.com/inox-project/inox/internal/core"
	"github.com/inox-project/inox/internal/utils"
)

var METHODS_WITH_NO_BODY = []string{"GET", "HEAD", "OPTIONS"}

const DEFAULT_ACCEPT_HEADER = "*/*"

// HttpRequest is considered immutable from the viewpoint of Inox code, it should NOT be mutated.
type HttpRequest struct {
	core.NoReprMixin
	core.NotClonableMixin

	isClientSide bool

	//accessible from inox
	Method             core.Str
	URL                core.URL
	Path               core.Path
	Body               *core.Reader
	Cookies            []*http.Cookie
	ParsedAcceptHeader mimeheader.AcceptHeader
	AcceptHeader       string
	ContentType        mimeheader.MimeType
	Session            *Session
	NewSession         bool

	headers     *core.Record //not set by default
	headersLock sync.Mutex

	//
	request *http.Request
}

func (req *HttpRequest) Request() *http.Request {
	return req.request
}

func (req *HttpRequest) IsGetOrHead() bool {
	return req.Method == "GET" || req.Method == "HEAD"
}

func (req *HttpRequest) AcceptAny() bool {
	for _, h := range req.ParsedAcceptHeader.MHeaders {
		if h.MimeType.Type == "*" && h.MimeType.Subtype == "*" {
			return true
		}
	}
	return false
}

func (req *HttpRequest) GetGoMethod(name string) (*core.GoFunction, bool) {
	return nil, false
}

func (req *HttpRequest) Prop(ctx *core.Context, name string) core.Value {
	switch name {
	case "method":
		return req.Method
	case "url":
		return req.URL
	case "path":
		return req.Path
	case "body":
		return req.Body
	case "headers":
		req.headersLock.Lock()
		defer req.headersLock.Unlock()
		if req.headers != nil {
			return req.headers
		}
		keys := make([]string, len(req.request.Header))
		vals := make([]core.Value, len(req.request.Header))

		i := 0
		for name, headerValues := range req.request.Header {
			keys[i] = name

			singleHeaderValues := make([]core.Value, len(headerValues))
			for i, val := range headerValues {
				singleHeaderValues[i] = core.Str(val)
			}

			vals[i] = core.NewTuple(singleHeaderValues)
			i++
		}
		req.headers = core.NewRecordFromKeyValLists(keys, vals)
		return req.headers
	case "cookies":
		//TODO
		return nil
	default:
		method, ok := req.GetGoMethod(name)
		if !ok {
			panic(core.FormatErrPropertyDoesNotExist(name, req))
		}
		return method
	}
}

func (*HttpRequest) SetProp(ctx *core.Context, name string, value core.Value) error {
	return core.ErrCannotSetProp
}

func (*HttpRequest) PropertyNames(ctx *core.Context) []string {
	return []string{"method", "url", "path", "body", "cookies", "headers"}
}

func NewClientSideRequest(r *http.Request) (*HttpRequest, error) {
	u := r.URL.String()

	if !strings.Contains(u, "://") {
		return nil, fmt.Errorf("cannot resolve URL of client side request")
	}

	return &HttpRequest{
		request:      r,
		isClientSide: true,
		URL:          core.URL(u),
	}, nil
}

func NewServerSideRequest(r *http.Request, logger *log.Logger, server *HttpServer) (*HttpRequest, error) {

	// method
	method := r.Method
	if method == "" {
		method = "GET"
	}

	// full URL
	url := r.URL.String()
	if !strings.Contains(url, "://") {
		if server == nil {
			return nil, fmt.Errorf("cannot resolve URL of request")
		}
		url = string(server.host) + url
	}

	// Content-Type header
	var contentType mimeheader.MimeType
	if !utils.SliceContains(METHODS_WITH_NO_BODY, string(method)) {
		mtype, err := mimeheader.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil {
			return nil, fmt.Errorf("invalid request: %w", err)
		}
		contentType = mtype
	}

	// Accept header
	acceptHeaderValue := r.Header.Get("Accept")
	if acceptHeaderValue == "" {
		acceptHeaderValue = DEFAULT_ACCEPT_HEADER
	}

	req := &HttpRequest{
		Method:             core.Str(method),
		URL:                core.URL(url),
		Path:               core.Path(r.URL.Path),
		Body:               core.WrapReader(r.Body, nil),
		Cookies:            r.Cookies(),
		request:            r,
		ParsedAcceptHeader: mimeheader.ParseAcceptHeader(acceptHeaderValue),
		AcceptHeader:       acceptHeaderValue,
		ContentType:        contentType,
	}

	session, err := getSession(req.request)
	if err == nil {
		req.Session = session
	} else if err == ErrSessionNotFound {
		logger.Println("no session id found, create new one")
		req.Session = addNewSession(server)
		req.NewSession = true
	} else {
		return nil, err
	}

	return req, nil
}
