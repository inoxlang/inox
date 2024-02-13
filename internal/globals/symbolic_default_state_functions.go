package internal

import (
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/symbolic"
	_http_symbolic "github.com/inoxlang/inox/internal/globals/http_ns/symbolic"

	"github.com/inoxlang/inox/internal/utils"
)

var (
	RAND_FN_PARAMS = []symbolic.Value{
		symbolic.NewMultivalue(symbolic.ANY_PATTERN, symbolic.ANY_INDEXABLE),
	}
	RAND_FN_PARAM_NAMES = []string{"arg"}

	TOSTR_FN_PARAMS = []symbolic.Value{
		symbolic.NewMultivalue(symbolic.ANY_BOOL, symbolic.ANY_INTEGRAL, symbolic.ANY_STR_LIKE, symbolic.ANY_BYTES_LIKE,
			symbolic.ANY_RUNE, symbolic.ANY_RES_NAME)}

	TOSTR_FN_PARAM_NAMES = []string{"arg"}

	TOSTRING_FN_PARAMS = TOSTR_FN_PARAMS
)

func init() {

	core.RegisterSymbolicGoFunctions([]any{
		_get_current_tx, func(ctx *symbolic.Context) *symbolic.Transaction {
			return &symbolic.Transaction{}
		},
		core.StartNewTransaction, func(ctx *symbolic.Context, options ...*symbolic.Option) *symbolic.Transaction {
			return &symbolic.Transaction{}
		},
		_execute, func(ctx *symbolic.Context, args ...symbolic.Value) (*symbolic.String, *symbolic.Error) {
			return symbolic.ANY_STRING, nil
		},
		_sha1, func(ctx *symbolic.Context, arg symbolic.Readable) *symbolic.ByteSlice {
			return &symbolic.ByteSlice{} //TODO: set length when symbolic ByteSlice supports it.
		},
		_md5, func(ctx *symbolic.Context, arg symbolic.Readable) *symbolic.ByteSlice {
			return &symbolic.ByteSlice{} //TODO: set length when symbolic ByteSlice supports it.
		},
		_mkpath, func(ctx *symbolic.Context, arg symbolic.Value) *symbolic.Path {
			return symbolic.ANY_PATH
		},
		_make_path_pattern, func(ctx *symbolic.Context, arg symbolic.Value) *symbolic.PathPattern {
			return symbolic.ANY_PATH_PATTERN
		},
		_mkurl, func(ctx *symbolic.Context, arg symbolic.Value) *symbolic.URL {
			return symbolic.ANY_URL
		},

		_rand, func(ctx *symbolic.Context, arg symbolic.Value) symbolic.Value {
			ctx.SetSymbolicGoFunctionParameters(&RAND_FN_PARAMS, RAND_FN_PARAM_NAMES)
			if patt, ok := arg.(symbolic.Pattern); ok {
				return patt.SymbolicValue()
			}
			if indexable, ok := arg.(symbolic.Indexable); ok {
				return indexable.Element()
			}
			return symbolic.ANY
		},

		_print, func(ctx *symbolic.Context, arg ...symbolic.Value) {},
		_fprint, func(ctx *symbolic.Context, out symbolic.Writable, arg ...symbolic.Value) {},
		_Error, func(ctx *symbolic.Context, s *symbolic.String, args ...symbolic.Serializable) *symbolic.Error {
			if len(args) > 1 {
				ctx.AddSymbolicGoFunctionError("at most two arguments were expected")
			}
			if len(args) == 0 {
				return symbolic.NewError(symbolic.Nil)
			}

			if args[0].IsMutable() {
				ctx.AddSymbolicGoFunctionError("data provided to create error should be immutable")
			}
			return symbolic.NewError(args[0])
		},

		//resource
		_readResource, func(ctx *symbolic.Context, res symbolic.ResourceName, args ...symbolic.Value) (symbolic.Value, *symbolic.Error) {
			var result symbolic.Value = symbolic.ANY

			for _, arg := range args {
				switch v := arg.(type) {
				case *symbolic.Option:
					if name, ok := v.Name(); ok && name == "raw" {
						result = symbolic.ANY_BYTES_LIKE
					}
				default:
				}
			}

			return result, nil
		},
		_getResource, func(*symbolic.Context, symbolic.ResourceName, ...symbolic.Value) (symbolic.Value, *symbolic.Error) {
			return symbolic.ANY, nil
		},
		_createResource, func(ctx *symbolic.Context, resource symbolic.ResourceName, args ...symbolic.Value) (symbolic.Value, *symbolic.Error) {
			switch resource.(type) {
			case *symbolic.Path:
				return nil, symbolic.ANY_ERR
			case *symbolic.URL:
				return _http_symbolic.ANY_RESP, nil
			default:
				ctx.AddFormattedSymbolicGoFunctionError("provided resource type is not supported: %s", symbolic.Stringify(resource))
			}
			return symbolic.ANY, nil
		},
		_updateResource, func(*symbolic.Context, symbolic.ResourceName, ...symbolic.Value) (symbolic.Value, *symbolic.Error) {
			return symbolic.ANY, nil
		},
		_deleteResource, func(*symbolic.Context, symbolic.ResourceName, ...symbolic.Value) (symbolic.Value, *symbolic.Error) {
			return symbolic.ANY, nil
		},

		//serve

		_serve, func(*symbolic.Context, symbolic.ResourceName) *symbolic.Error {
			return nil
		},

		//
		_typeof, func(ctx *symbolic.Context, arg symbolic.Value) *symbolic.Type {
			return symbolic.ANY_TYPE
		},

		encodeBase64, func(ctx *symbolic.Context, arg symbolic.Readable) *symbolic.String {
			return symbolic.ANY_STRING
		},

		decodeBase64, func(ctx *symbolic.Context, arg symbolic.Readable) (*symbolic.ByteSlice, *symbolic.Error) {
			return symbolic.ANY_BYTE_SLICE, nil
		},

		encodeHex, func(ctx *symbolic.Context, arg symbolic.Readable) *symbolic.String {
			return symbolic.ANY_STRING
		},

		decodeHex, func(ctx *symbolic.Context, arg symbolic.Readable) (*symbolic.ByteSlice, *symbolic.Error) {
			return symbolic.ANY_BYTE_SLICE, nil
		},

		_tostr, func(ctx *symbolic.Context, arg symbolic.Value) symbolic.StringLike {
			ctx.SetSymbolicGoFunctionParameters(&TOSTR_FN_PARAMS, TOSTR_FN_PARAM_NAMES)
			return symbolic.ANY_STR_LIKE
		},
		_tostring, func(ctx *symbolic.Context, arg symbolic.Value) *symbolic.String {
			ctx.SetSymbolicGoFunctionParameters(&TOSTRING_FN_PARAMS, TOSTR_FN_PARAM_NAMES)
			return symbolic.ANY_STRING
		},
		_torune, func(ctx *symbolic.Context, arg symbolic.Integral) *symbolic.Rune {
			return symbolic.ANY_RUNE
		},
		_tobyte, func(ctx *symbolic.Context, arg *symbolic.Int) *symbolic.Byte {
			return symbolic.ANY_BYTE
		},
		_tofloat, func(ctx *symbolic.Context, arg symbolic.Integral) *symbolic.Float {
			return symbolic.ANY_FLOAT
		},
		_toint, func(ctx *symbolic.Context, arg symbolic.Value) *symbolic.Int {
			switch arg.(type) {
			case *symbolic.Float, symbolic.Integral:
			default:
				ctx.AddFormattedSymbolicGoFunctionError("toint only accepts floats & integral values, type is %s", symbolic.Stringify(arg))
			}
			return symbolic.ANY_INT
		},
		_tobytecount, func(ctx *symbolic.Context, v *symbolic.Int) *symbolic.ByteCount {
			if v.HasValue() {
				if v.Value() < 0 {
					ctx.AddFormattedSymbolicGoFunctionError("only positives values are allowed")
				}
				return symbolic.NewByteCount(v.Value())
			}

			return symbolic.ANY_BYTECOUNT
		},
		_torstream, func(ctx *symbolic.Context, arg symbolic.Value) *symbolic.ReadableStream {
			return symbolic.NewReadableStream(symbolic.ANY)
		},

		//

		core.ToJSON, func(ctx *symbolic.Context, arg symbolic.Value, pattern *symbolic.OptionalParam[symbolic.Pattern]) *symbolic.String {
			return symbolic.ANY_STRING
		},
		core.ToPrettyJSON, func(ctx *symbolic.Context, arg symbolic.Value, pattern *symbolic.OptionalParam[symbolic.Pattern]) *symbolic.String {
			return symbolic.ANY_STRING
		},
		core.AsJSON, func(ctx *symbolic.Context, v symbolic.Serializable) *symbolic.String {
			//TODO: recursively check that $v contains supported values.
			return symbolic.ANY_STRING
		},

		_parse, func(ctx *symbolic.Context, arg symbolic.Readable, p symbolic.Pattern) (symbolic.Value, *symbolic.Error) {
			return p.SymbolicValue(), nil
		},
		_split, func(ctx *symbolic.Context, arg symbolic.Readable, sep *symbolic.String, p *symbolic.OptionalParam[symbolic.Pattern]) (symbolic.Value, *symbolic.Error) {
			if p.Value == nil {
				return symbolic.STRLIKE_LIST, nil
			}

			serializable, ok := (*p.Value).SymbolicValue().(symbolic.Serializable)
			if !ok {
				serializable = symbolic.ANY_SERIALIZABLE
			}

			return symbolic.NewListOf(serializable), nil
		},

		_idt, func(ctx *symbolic.Context, arg symbolic.Value) symbolic.Value {
			return arg
		},
		_len, func(ctx *symbolic.Context, arg symbolic.Indexable) *symbolic.Int {
			return symbolic.ANY_INT
		},
		_len_range, func(ctx *symbolic.Context, arg symbolic.StringPattern) *symbolic.IntRange {
			return symbolic.ANY_INT_RANGE
		},
		_is_mutable, func(ctx *symbolic.Context, arg symbolic.Value) *symbolic.Bool {
			return symbolic.ANY_BOOL
		},

		_mkbytes, func(ctx *symbolic.Context, size *symbolic.ByteCount) *symbolic.ByteSlice {
			return symbolic.ANY_BYTE_SLICE
		},
		_Runes, func(ctx *symbolic.Context, arg symbolic.Readable) *symbolic.RuneSlice {
			return symbolic.ANY_RUNE_SLICE
		},
		_EmailAddress, func(ctx *symbolic.Context, arg symbolic.Readable) *symbolic.EmailAddress {
			return symbolic.ANY_EMAIL_ADDR
		},
		_ULID, func(ctx *symbolic.Context, arg symbolic.Readable) *symbolic.ULID {
			return symbolic.ANY_ULID
		},
		_UUIDV4, func(ctx *symbolic.Context, arg symbolic.Readable) *symbolic.UUIDv4 {
			return symbolic.ANY_UUIDv4
		},
		_Bytes, func(ctx *symbolic.Context, arg symbolic.Readable) *symbolic.ByteSlice {
			return symbolic.ANY_BYTE_SLICE
		},
		_Reader, func(ctx *symbolic.Context, arg symbolic.Readable) *symbolic.Reader {
			return symbolic.ANY_READER
		},

		_dynimport, func(ctx *symbolic.Context, src symbolic.Value, argObj *symbolic.Object, manifestObj *symbolic.Object, options ...symbolic.Value) (*symbolic.LThread, *symbolic.Error) {
			return &symbolic.LThread{}, nil
		},
		_run, func(ctx *symbolic.Context, src *symbolic.Path, args ...symbolic.Value) *symbolic.Error {
			return nil
		},
		_is_space, func(ctx *symbolic.Context, s *symbolic.Rune) *symbolic.Bool {
			return symbolic.ANY_BOOL
		},
		_is_even, func(ctx *symbolic.Context, i *symbolic.Int) *symbolic.Bool {
			return symbolic.ANY_BOOL
		},
		_is_odd, func(ctx *symbolic.Context, i *symbolic.Int) *symbolic.Bool {
			return symbolic.ANY_BOOL
		},
		//

		core.NewEventSource, func(ctx *symbolic.Context, resourceNameOrPattern symbolic.Value) (*symbolic.EventSource, *symbolic.Error) {
			return symbolic.NewEventSource(), nil
		},

		_cancel_exec, func(ctx *symbolic.Context) {

		},

		_url_of, func(ctx *symbolic.Context, v symbolic.Value) *symbolic.URL {
			if urlHolder, ok := v.(symbolic.UrlHolder); ok {
				url, ok := urlHolder.URL()
				if ok {
					return url
				}
			}
			return symbolic.ANY_URL
		},
		//

		core.SumOptions, func(ctx *symbolic.Context, config *symbolic.Object, options ...*symbolic.Option) (symbolic.Value, *symbolic.Error) {
			return symbolic.NewMultivalue(symbolic.ANY_OBJ, symbolic.Nil), nil
		},

		_List, func(ctx *symbolic.Context, args ...symbolic.Value) *symbolic.List {
			return symbolic.NewListOf(symbolic.ANY_SERIALIZABLE)
		},
		_Event, func(ctx *symbolic.Context, value symbolic.Value) *symbolic.Event {
			event, err := symbolic.NewEvent(value)
			if err != nil {
				ctx.AddSymbolicGoFunctionError(err.Error())
				return utils.Must(symbolic.NewEvent(symbolic.ANY))
			}
			return event
		},

		// protocol
		setClientForURL, func(ctx *symbolic.Context, u *symbolic.URL, client symbolic.ProtocolClient) *symbolic.Error {
			return nil
		},

		setClientForHost, func(ctx *symbolic.Context, h *symbolic.Host, client symbolic.ProtocolClient) *symbolic.Error {
			return nil
		},

		//
		_Color, func(ctx *symbolic.Context, firstArg symbolic.Value, others ...symbolic.Value) *symbolic.Color {
			return symbolic.ANY_COLOR
		},

		_add_ctx_data, func(ctx *symbolic.Context, path *symbolic.Path, value symbolic.Value) {

		},
		_ctx_data, func(ctx *symbolic.Context, path *symbolic.Path, pattern *symbolic.OptionalParam[symbolic.Pattern]) symbolic.Value {
			if pattern == nil || pattern.Value == nil {
				return symbolic.ANY
			}

			return (*pattern.Value).SymbolicValue()
		},
		_get_system_graph, func(ctx *symbolic.Context) (*symbolic.SystemGraph, *symbolic.Bool) {
			return symbolic.ANY_SYSTEM_GRAPH, symbolic.ANY_BOOL
		},

		_propnames, func(ctx *symbolic.Context, v symbolic.Value) *symbolic.List {
			if _, ok := v.(symbolic.IProps); !ok {
				ctx.AddSymbolicGoFunctionError("value cannot have properties")
			}
			return symbolic.NewListOf(symbolic.ANY_STRING)
		},
		_get, func(ctx *symbolic.Context, url *symbolic.URL) (symbolic.Serializable, *symbolic.Error) {
			v, err := symbolic.GetValueAtURL(url, ctx.EvalState())
			if err != nil {
				ctx.AddSymbolicGoFunctionError(err.Error())
				return symbolic.ANY_SERIALIZABLE, nil
			}
			return v, nil
		},
	})

}
