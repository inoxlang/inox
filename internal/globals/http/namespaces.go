package internal

import (
	"reflect"

	core "github.com/inoxlang/inox/internal/core"
	symbolic "github.com/inoxlang/inox/internal/core/symbolic"
	http_symbolic "github.com/inoxlang/inox/internal/globals/http/symbolic"

	help "github.com/inoxlang/inox/internal/globals/help"
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
		HttpRead, func(ctx *symbolic.Context, u *symbolic.URL, args ...symbolic.SymbolicValue) (symbolic.SymbolicValue, *symbolic.Error) {
			return symbolic.ANY, nil
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
		PercentEncode, func(ctx *symbolic.Context, s symbolic.StringLike) symbolic.StringLike {
			return symbolic.ANY_STR_LIKE
		},
		PercentDecode, func(ctx *symbolic.Context, s symbolic.StringLike) (symbolic.StringLike, *symbolic.Error) {
			return symbolic.ANY_STR_LIKE, nil
		},
	})

	help.RegisterHelpValues(map[string]any{
		"http.exists":     httpExists,
		"http.get":        HttpGet,
		"http.read":       HttpRead,
		"http.post":       HttpPost,
		"http.patch":      HttpPatch,
		"http.delete":     HttpDelete,
		"http.Server":     NewHttpServer,
		"http.FileServer": NewFileServer,
		"http.servefile":  serveFile,
		"http.Client":     NewClient,
	})
}

func NewHttpNamespace() *core.Record {
	return core.NewRecordFromMap(core.ValMap{
		"exists":         core.WrapGoFunction(httpExists),
		"get":            core.WrapGoFunction(HttpGet),
		"read":           core.WrapGoFunction(HttpRead),
		"post":           core.WrapGoFunction(HttpPost),
		"patch":          core.WrapGoFunction(HttpPatch),
		"delete":         core.WrapGoFunction(HttpDelete),
		"Server":         core.WrapGoFunction(NewHttpServer),
		"FileServer":     core.WrapGoFunction(NewFileServer),
		"servefile":      core.WrapGoFunction(serveFile),
		"Client":         core.WrapGoFunction(NewClient),
		"percent_encode": core.WrapGoFunction(PercentEncode),
		"percent_decode": core.WrapGoFunction(PercentDecode),
	})
}
