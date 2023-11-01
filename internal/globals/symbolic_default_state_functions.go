package internal

import (
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/symbolic"
	_http_symbolic "github.com/inoxlang/inox/internal/globals/http_ns/symbolic"

	"github.com/inoxlang/inox/internal/utils"
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
			return &symbolic.String{}, nil
		},
		_sha1, func(ctx *symbolic.Context, arg symbolic.Readable) *symbolic.ByteSlice {
			return &symbolic.ByteSlice{}
		},
		_sha2, func(ctx *symbolic.Context, arg symbolic.Readable) *symbolic.ByteSlice {
			return &symbolic.ByteSlice{}
		},
		_mkpath, func(ctx *symbolic.Context, arg symbolic.Value) *symbolic.Path {
			return &symbolic.Path{}
		},
		_make_path_pattern, func(ctx *symbolic.Context, arg symbolic.Value) *symbolic.PathPattern {
			return &symbolic.PathPattern{}
		},
		_mkurl, func(ctx *symbolic.Context, arg symbolic.Value) *symbolic.URL {
			return &symbolic.URL{}
		},

		_rand, func(ctx *symbolic.Context, arg symbolic.Value) symbolic.Value {
			return symbolic.ANY
		},

		_logvals, func(ctx *symbolic.Context, arg ...symbolic.Value) {},
		_log, func(ctx *symbolic.Context, arg ...symbolic.Value) {},
		_print, func(ctx *symbolic.Context, arg ...symbolic.Value) {},
		_fprint, func(ctx *symbolic.Context, out symbolic.Writable, arg ...symbolic.Value) {},
		_printvals, func(ctx *symbolic.Context, arg ...symbolic.Value) {},
		_stringify_ast, func(ctx *symbolic.Context, arg *symbolic.AstNode) {},
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
			return &symbolic.Type{}
		},

		encodeBase64, func(ctx *symbolic.Context, arg symbolic.Readable) *symbolic.String {
			return symbolic.ANY_STR
		},

		decodeBase64, func(ctx *symbolic.Context, arg symbolic.Readable) (*symbolic.ByteSlice, *symbolic.Error) {
			return symbolic.ANY_BYTE_SLICE, nil
		},

		encodeHex, func(ctx *symbolic.Context, arg symbolic.Readable) *symbolic.String {
			return symbolic.ANY_STR
		},

		decodeHex, func(ctx *symbolic.Context, arg symbolic.Readable) (*symbolic.ByteSlice, *symbolic.Error) {
			return symbolic.ANY_BYTE_SLICE, nil
		},

		_tostr, func(ctx *symbolic.Context, arg symbolic.Value) symbolic.StringLike {
			return symbolic.ANY_STR_LIKE
		},
		_torune, func(ctx *symbolic.Context, arg symbolic.Integral) *symbolic.Rune {
			return symbolic.ANY_RUNE
		},
		_tobyte, func(ctx *symbolic.Context, arg *symbolic.Int) *symbolic.Byte {
			return symbolic.ANY_BYTE
		},
		_tofloat, func(ctx *symbolic.Context, arg *symbolic.Int) *symbolic.Float {
			return symbolic.ANY_FLOAT
		},
		_toint, func(ctx *symbolic.Context, arg symbolic.Value) *symbolic.Int {
			switch arg.(type) {
			case *symbolic.Float, *symbolic.Byte:
			default:
				ctx.AddFormattedSymbolicGoFunctionError("toint only accepts floats & bytes, type is %s", symbolic.Stringify(arg))
			}
			return symbolic.ANY_INT
		},
		_torstream, func(ctx *symbolic.Context, arg symbolic.Value) *symbolic.ReadableStream {
			return symbolic.NewReadableStream(symbolic.ANY)
		},

		//

		core.ToJSON, func(ctx *symbolic.Context, arg symbolic.Value) *symbolic.String {
			return &symbolic.String{}
		},
		core.ToPrettyJSON, func(ctx *symbolic.Context, arg symbolic.Value) *symbolic.String {
			return &symbolic.String{}
		},

		_repr, func(ctx *symbolic.Context, arg symbolic.Serializable) *symbolic.String {
			return &symbolic.String{}
		},
		_parse_repr, func(ctx *symbolic.Context, arg symbolic.Readable) (symbolic.Value, *symbolic.Error) {
			return symbolic.ANY, nil
		},
		_parse, func(ctx *symbolic.Context, arg symbolic.Readable, p symbolic.Pattern) (symbolic.Value, *symbolic.Error) {
			return p.SymbolicValue(), nil
		},
		_split, func(ctx *symbolic.Context, arg symbolic.Readable, sep *symbolic.String, p symbolic.Pattern) (symbolic.Value, *symbolic.Error) {
			return symbolic.NewListOf(p.SymbolicValue().(symbolic.Serializable)), nil
		},

		_idt, func(ctx *symbolic.Context, arg symbolic.Value) symbolic.Value {
			return arg
		},
		_len, func(ctx *symbolic.Context, arg symbolic.Indexable) *symbolic.Int {
			return symbolic.ANY_INT
		},
		_len_range, func(ctx *symbolic.Context, arg symbolic.StringPattern) *symbolic.IntRange {
			return &symbolic.IntRange{}
		},

		_mkbytes, func(ctx *symbolic.Context, size *symbolic.Int) *symbolic.ByteSlice {
			return &symbolic.ByteSlice{}
		},
		_Runes, func(ctx *symbolic.Context, arg symbolic.Readable) *symbolic.RuneSlice {
			return &symbolic.RuneSlice{}
		},
		_Bytes, func(ctx *symbolic.Context, arg symbolic.Readable) *symbolic.ByteSlice {
			return &symbolic.ByteSlice{}
		},
		_Reader, func(ctx *symbolic.Context, arg symbolic.Readable) *symbolic.Reader {
			return &symbolic.Reader{}
		},

		_dynimport, func(ctx *symbolic.Context, src symbolic.Value, argObj *symbolic.Object, manifestObj *symbolic.Object, options ...symbolic.Value) (*symbolic.LThread, *symbolic.Error) {
			return &symbolic.LThread{}, nil
		},
		_run, func(ctx *symbolic.Context, src *symbolic.Path, args ...symbolic.Value) *symbolic.Error {
			return nil
		},
		_is_rune_space, func(ctx *symbolic.Context, s *symbolic.Rune) *symbolic.Bool {
			return &symbolic.Bool{}
		},
		_is_even, func(ctx *symbolic.Context, i *symbolic.Int) *symbolic.Bool {
			return &symbolic.Bool{}
		},
		_is_odd, func(ctx *symbolic.Context, i *symbolic.Int) *symbolic.Bool {
			return &symbolic.Bool{}
		},
		//

		core.NewEventSource, func(ctx *symbolic.Context, resourceNameOrPattern symbolic.Value) (*symbolic.EventSource, *symbolic.Error) {
			return symbolic.NewEventSource(), nil
		},

		_cancel_exec, func(ctx *symbolic.Context) {

		},

		_url_of, func(ctx *symbolic.Context, v symbolic.Value) *symbolic.URL {
			return &symbolic.URL{}
		},
		//

		core.SumOptions, func(ctx *symbolic.Context, config *symbolic.Object, options ...*symbolic.Option) (*symbolic.Object, *symbolic.Error) {
			return symbolic.NewAnyObject(), nil
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

		_add_ctx_data, func(ctx *symbolic.Context, name *symbolic.Identifier, value symbolic.Value) {

		},
		_ctx_data, func(ctx *symbolic.Context, name *symbolic.Identifier) symbolic.Value {
			return symbolic.ANY
		},
		_get_system_graph, func(ctx *symbolic.Context) (*symbolic.SystemGraph, *symbolic.Bool) {
			return symbolic.ANY_SYSTEM_GRAPH, symbolic.ANY_BOOL
		},

		_propnames, func(ctx *symbolic.Context, v symbolic.Value) *symbolic.List {
			if _, ok := v.(symbolic.IProps); !ok {
				ctx.AddSymbolicGoFunctionError("value cannot have properties")
			}
			return symbolic.NewListOf(symbolic.ANY_STR)
		},
	})

}
