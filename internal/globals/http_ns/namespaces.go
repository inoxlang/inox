package http_ns

import (
	"reflect"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/permkind"
	"github.com/inoxlang/inox/internal/core/symbolic"
	http_symbolic "github.com/inoxlang/inox/internal/globals/http_ns/symbolic"
	"github.com/inoxlang/inox/internal/help"
	"github.com/inoxlang/inox/internal/utils"
)

const (
	HTTP_READ_PERM_MIGHT_BE_MISSING    = "http read permission might be missing"
	HTTP_WRITE_PERM_MIGHT_BE_MISSING   = "http write permission might be missing"
	HTTP_DELETE_PERM_MIGHT_BE_MISSING  = "http delete permission might be missing"
	HTTP_PROVIDE_PERM_MIGHT_BE_MISSING = "http provide permission might be missing"
)

func init() {
	//register limits
	core.RegisterLimit(HTTP_REQUEST_RATE_LIMIT_NAME, core.SimpleRateLimit, 0)
	core.RegisterLimit(HTTP_UPLOAD_RATE_LIMIT_NAME, core.ByteRateLimit, 0)

	//register patterns
	core.RegisterDefaultPatternNamespace("http", &core.PatternNamespace{
		Patterns: map[string]core.Pattern{
			"resp-writer": &core.TypePattern{
				Name:          "http.resp-writer",
				Type:          reflect.TypeOf(&HttpResponseWriter{}),
				SymbolicValue: &http_symbolic.HttpResponseWriter{},
			},
			"req": CALLABLE_HTTP_REQUEST_PATTERN,
			"method": core.NewUnionPattern(utils.MapSlice(METHODS, func(s string) core.Pattern {
				return core.NewExactValuePattern(core.Identifier(s))
			}), nil),
		},
	})

	stringOrStringList := symbolic.AsSerializableChecked(symbolic.NewMultivalue(
		symbolic.NewListOf(symbolic.ANY_STR_LIKE),
		symbolic.ANY_STR_LIKE,
	))

	// register symbolic version of Go functions
	core.RegisterSymbolicGoFunctions([]any{
		httpExists, func(ctx *symbolic.Context, arg symbolic.Value) *symbolic.Bool {
			if !ctx.HasAPermissionWithKindAndType(permkind.Read, permkind.HTTP_PERM_TYPENAME) {
				ctx.AddSymbolicGoFunctionWarning(HTTP_READ_PERM_MIGHT_BE_MISSING)
			}
			return symbolic.ANY_BOOL
		},
		HttpGet, func(ctx *symbolic.Context, u *symbolic.URL, args ...symbolic.Value) (*http_symbolic.HttpResponse, *symbolic.Error) {
			if !ctx.HasAPermissionWithKindAndType(permkind.Read, permkind.HTTP_PERM_TYPENAME) {
				ctx.AddSymbolicGoFunctionWarning(HTTP_READ_PERM_MIGHT_BE_MISSING)
			}
			return http_symbolic.ANY_RESP, nil
		},
		HttpRead, func(ctx *symbolic.Context, u *symbolic.URL, args ...symbolic.Value) (symbolic.Value, *symbolic.Error) {
			if !ctx.HasAPermissionWithKindAndType(permkind.Read, permkind.HTTP_PERM_TYPENAME) {
				ctx.AddSymbolicGoFunctionWarning(HTTP_READ_PERM_MIGHT_BE_MISSING)
			}
			return symbolic.ANY, nil
		},
		HttpPost, func(ctx *symbolic.Context, args ...symbolic.Value) (*http_symbolic.HttpResponse, *symbolic.Error) {
			if !ctx.HasAPermissionWithKindAndType(permkind.Write, permkind.HTTP_PERM_TYPENAME) {
				ctx.AddSymbolicGoFunctionWarning(HTTP_WRITE_PERM_MIGHT_BE_MISSING)
			}
			return http_symbolic.ANY_RESP, nil
		},
		HttpPatch, func(ctx *symbolic.Context, args ...symbolic.Value) (*http_symbolic.HttpResponse, *symbolic.Error) {
			if !ctx.HasAPermissionWithKindAndType(permkind.Write, permkind.HTTP_PERM_TYPENAME) {
				ctx.AddSymbolicGoFunctionWarning(HTTP_WRITE_PERM_MIGHT_BE_MISSING)
			}
			return http_symbolic.ANY_RESP, nil
		},
		HttpDelete, func(ctx *symbolic.Context, args ...symbolic.Value) (*http_symbolic.HttpResponse, *symbolic.Error) {
			if !ctx.HasAPermissionWithKindAndType(permkind.Delete, permkind.HTTP_PERM_TYPENAME) {
				ctx.AddSymbolicGoFunctionWarning(HTTP_DELETE_PERM_MIGHT_BE_MISSING)
			}
			return http_symbolic.ANY_RESP, nil
		},
		NewHttpsServer, newSymbolicHttpsServer,
		NewFileServer, func(ctx *symbolic.Context, args ...symbolic.Value) (*http_symbolic.HttpServer, *symbolic.Error) {
			if !ctx.HasAPermissionWithKindAndType(permkind.Provide, permkind.HTTP_PERM_TYPENAME) {
				ctx.AddSymbolicGoFunctionWarning(HTTP_PROVIDE_PERM_MIGHT_BE_MISSING)
			}
			return &http_symbolic.HttpServer{}, nil
		},
		ServeFile, func(ctx *symbolic.Context, rw *http_symbolic.HttpResponseWriter, r *http_symbolic.HttpRequest, path *symbolic.Path) *symbolic.Error {
			return nil
		},
		Mime_, func(ctx *symbolic.Context, arg *symbolic.String) (*symbolic.Mimetype, *symbolic.Error) {
			return &symbolic.Mimetype{}, nil
		},
		core.UrlOf, func(ctx *symbolic.Context, v symbolic.Value) symbolic.Value {
			return symbolic.ANY
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
		NewCSP, func(ctx *symbolic.Context, desc *symbolic.Object) (*http_symbolic.ContentSecurityPolicy, *symbolic.Error) {
			ctx.SetSymbolicGoFunctionParameters(&[]symbolic.Value{
				symbolic.NewInexactObject(map[string]symbolic.Serializable{
					"default-src":     stringOrStringList,
					"frame-ancestors": stringOrStringList,
					"frame-src":       stringOrStringList,
					"script-src":      stringOrStringList,
					"script-src-elem": stringOrStringList,
					"script-src-attr": stringOrStringList,
					"worker-src":      stringOrStringList,
					"connect-src":     stringOrStringList,
					"font-src":        stringOrStringList,
					"img-src":         stringOrStringList,
					"media-src":       stringOrStringList,
					"style-src":       stringOrStringList,
					"style-src-attr":  stringOrStringList,
					"style-src-elem":  stringOrStringList,
				}, map[string]struct{}{
					"default-src":     {},
					"frame-ancestors": {},
					"frame-src":       {},
					"script-src":      {},
					"script-src-elem": {},
					"script-src-attr": {},
					"worker-src":      {},
					"connect-src":     {},
					"font-src":        {},
					"img-src":         {},
					"media-src":       {},
					"style-src":       {},
					"style-src-attr":  {},
					"style-src-elem":  {},
				}, nil),
			}, []string{"csp"})

			return http_symbolic.NewCSP(), nil
		},
	})

	help.RegisterHelpValues(map[string]any{
		"http.exists":     httpExists,
		"http.get":        HttpGet,
		"http.read":       HttpRead,
		"http.post":       HttpPost,
		"http.patch":      HttpPatch,
		"http.delete":     HttpDelete,
		"http.Server":     NewHttpsServer,
		"http.FileServer": NewFileServer,
		"http.servefile":  ServeFile,
		"http.Client":     NewClient,
		"http.CSP":        NewCSP,
	})
}

func NewHttpNamespace() *core.Namespace {
	return core.NewNamespace("http", map[string]core.Value{
		"exists":         core.WrapGoFunction(httpExists),
		"get":            core.WrapGoFunction(HttpGet),
		"read":           core.WrapGoFunction(HttpRead),
		"post":           core.WrapGoFunction(HttpPost),
		"patch":          core.WrapGoFunction(HttpPatch),
		"delete":         core.WrapGoFunction(HttpDelete),
		"Server":         core.WrapGoFunction(NewHttpsServer),
		"FileServer":     core.WrapGoFunction(NewFileServer),
		"servefile":      core.WrapGoFunction(ServeFile),
		"Client":         core.WrapGoFunction(NewClient),
		"percent_encode": core.WrapGoFunction(PercentEncode),
		"percent_decode": core.WrapGoFunction(PercentDecode),
		"CSP":            core.WrapGoFunction(NewCSP),
	})
}
