package internal

import (
	"reflect"

	core "github.com/inoxlang/inox/internal/core"
	symbolic "github.com/inoxlang/inox/internal/core/symbolic"
	http_symbolic "github.com/inoxlang/inox/internal/globals/http/symbolic"
)

func init() {
	//register limitations
	core.LimRegistry.RegisterLimitation(HTTP_REQUEST_RATE_LIMIT_NAME, core.SimpleRateLimitation, 0)
	core.LimRegistry.RegisterLimitation(HTTP_UPLOAD_RATE_LIMIT_NAME, core.ByteRateLimitation, 0)

	//register patterns
	core.RegisterDefaultPatternNamespace("http", &core.PatternNamespace{
		Patterns: map[string]core.Pattern{
			"resp_writer": &core.TypePattern{
				Name:          "http.resp_writer",
				Type:          reflect.TypeOf(&HttpResponseWriter{}),
				SymbolicValue: &http_symbolic.HttpResponseWriter{},
			},
			"req": &core.TypePattern{
				Name:          "http.req",
				Type:          reflect.TypeOf(&HttpRequest{}),
				SymbolicValue: &http_symbolic.HttpRequest{},
			},
		},
	})

	// register symbolic version of Go functions
	core.RegisterSymbolicGoFunctions([]any{
		httpExists, func(ctx *symbolic.Context, arg symbolic.SymbolicValue) *symbolic.Bool {
			return &symbolic.Bool{}
		},
		HttpGet, func(ctx *symbolic.Context, u *symbolic.URL, args ...symbolic.SymbolicValue) (*http_symbolic.HttpResponse, *symbolic.Error) {
			return &http_symbolic.HttpResponse{}, nil
		},
		httpGetBody, func(ctx *symbolic.Context, u *symbolic.URL, args ...symbolic.SymbolicValue) (*symbolic.ByteSlice, *symbolic.Error) {
			return &symbolic.ByteSlice{}, nil
		},
		HttpPost, func(ctx *symbolic.Context, args ...symbolic.SymbolicValue) (*http_symbolic.HttpResponse, *symbolic.Error) {
			return &http_symbolic.HttpResponse{}, nil
		},
		HttpPatch, func(ctx *symbolic.Context, args ...symbolic.SymbolicValue) (*http_symbolic.HttpResponse, *symbolic.Error) {
			return &http_symbolic.HttpResponse{}, nil
		},
		HttpDelete, func(ctx *symbolic.Context, args ...symbolic.SymbolicValue) (*http_symbolic.HttpResponse, *symbolic.Error) {
			return &http_symbolic.HttpResponse{}, nil
		},
		NewHttpServer, func(ctx *symbolic.Context, ars ...symbolic.SymbolicValue) (*http_symbolic.HttpServer, *symbolic.Error) {
			return &http_symbolic.HttpServer{}, nil
		},
		NewFileServer, func(ctx *symbolic.Context, ars ...symbolic.SymbolicValue) (*http_symbolic.HttpServer, *symbolic.Error) {
			return &http_symbolic.HttpServer{}, nil
		},
		serveFile, func(ctx *symbolic.Context, rw *http_symbolic.HttpResponseWriter, r *http_symbolic.HttpRequest, path *symbolic.Path) *symbolic.Error {
			return nil
		},
		Mime_, func(ctx *symbolic.Context, arg *symbolic.String) (*symbolic.Mimetype, *symbolic.Error) {
			return &symbolic.Mimetype{}, nil
		},
		core.UrlOf, func(ctx *symbolic.Context, v symbolic.SymbolicValue) symbolic.SymbolicValue {
			return &symbolic.Any{}
		},
		NewClient, func(ctx *symbolic.Context, config *symbolic.Object) *http_symbolic.HttpClient {
			return &http_symbolic.HttpClient{}
		},
	})
}

func NewHttpNamespace() *core.Record {
	return core.NewRecordFromMap(core.ValMap{
		"exists":     core.ValOf(httpExists),
		"get":        core.ValOf(HttpGet),
		"getbody":    core.ValOf(httpGetBody),
		"post":       core.ValOf(HttpPost),
		"patch":      core.ValOf(HttpPatch),
		"delete":     core.ValOf(HttpDelete),
		"Server":     core.ValOf(NewHttpServer),
		"FileServer": core.ValOf(NewFileServer),
		"servefile":  core.ValOf(serveFile),
		"Client":     core.ValOf(NewClient),
	})
}
