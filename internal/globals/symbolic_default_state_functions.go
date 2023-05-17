package internal

import (
	core "github.com/inoxlang/inox/internal/core"
	symbolic "github.com/inoxlang/inox/internal/core/symbolic"
	_http_symbolic "github.com/inoxlang/inox/internal/globals/http/symbolic"

	"github.com/inoxlang/inox/internal/utils"
)

func init() {

	core.RegisterSymbolicGoFunctions([]any{
		_get_current_tx, func(ctx *symbolic.Context) *symbolic.Transaction {
			return &symbolic.Transaction{}
		},
		core.NewTransaction, func(ctx *symbolic.Context, options ...*symbolic.Option) *symbolic.Transaction {
			return &symbolic.Transaction{}
		},
		core.StartNewTransaction, func(ctx *symbolic.Context, options ...*symbolic.Option) *symbolic.Transaction {
			return &symbolic.Transaction{}
		},
		_execute, func(ctx *symbolic.Context, args ...symbolic.SymbolicValue) (*symbolic.String, *symbolic.Error) {
			return &symbolic.String{}, &symbolic.Error{}
		},
		_sha1, func(ctx *symbolic.Context, arg symbolic.Readable) *symbolic.ByteSlice {
			return &symbolic.ByteSlice{}
		},
		_sha2, func(ctx *symbolic.Context, arg symbolic.Readable) *symbolic.ByteSlice {
			return &symbolic.ByteSlice{}
		},
		_mkpath, func(ctx *symbolic.Context, arg symbolic.SymbolicValue) *symbolic.Path {
			return &symbolic.Path{}
		},
		_make_path_pattern, func(ctx *symbolic.Context, arg symbolic.SymbolicValue) *symbolic.PathPattern {
			return &symbolic.PathPattern{}
		},
		_mkurl, func(ctx *symbolic.Context, arg symbolic.SymbolicValue) *symbolic.URL {
			return &symbolic.URL{}
		},

		_rand, func(ctx *symbolic.Context, arg symbolic.SymbolicValue) symbolic.SymbolicValue {
			return symbolic.ANY
		},
		_clone_val, func(ctx *symbolic.Context, arg symbolic.SymbolicValue) symbolic.SymbolicValue {
			return arg
		},

		_logvals, func(ctx *symbolic.Context, arg ...symbolic.SymbolicValue) {},
		_log, func(ctx *symbolic.Context, arg ...symbolic.SymbolicValue) {},
		_print, func(ctx *symbolic.Context, arg ...symbolic.SymbolicValue) {},
		_fprint, func(ctx *symbolic.Context, out symbolic.Writable, arg ...symbolic.SymbolicValue) {},
		_printvals, func(ctx *symbolic.Context, arg ...symbolic.SymbolicValue) {},
		_stringify_ast, func(ctx *symbolic.Context, arg *symbolic.AstNode) {},
		_Error, func(ctx *symbolic.Context, s *symbolic.String, args ...symbolic.SymbolicValue) *symbolic.Error {
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
		_readResource, func(ctx *symbolic.Context, res symbolic.ResourceName, args ...symbolic.SymbolicValue) (symbolic.SymbolicValue, *symbolic.Error) {
			var result symbolic.SymbolicValue = symbolic.ANY

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
		_getResource, func(*symbolic.Context, symbolic.ResourceName, ...symbolic.SymbolicValue) (symbolic.SymbolicValue, *symbolic.Error) {
			return symbolic.ANY, nil
		},
		_createResource, func(ctx *symbolic.Context, resource symbolic.ResourceName, args ...symbolic.SymbolicValue) (symbolic.SymbolicValue, *symbolic.Error) {
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
		_updateResource, func(*symbolic.Context, symbolic.ResourceName, ...symbolic.SymbolicValue) (symbolic.SymbolicValue, *symbolic.Error) {
			return symbolic.ANY, nil
		},
		_deleteResource, func(*symbolic.Context, symbolic.ResourceName, ...symbolic.SymbolicValue) (symbolic.SymbolicValue, *symbolic.Error) {
			return symbolic.ANY, nil
		},

		//serve

		_serve, func(*symbolic.Context, symbolic.ResourceName) *symbolic.Error {
			return nil
		},

		//
		_typeof, func(ctx *symbolic.Context, arg symbolic.SymbolicValue) *symbolic.Type {
			return &symbolic.Type{}
		},

		encodeBase64, func(ctx *symbolic.Context, arg symbolic.Readable) *symbolic.String {
			return &symbolic.String{}
		},

		decodeBase64, func(ctx *symbolic.Context, arg symbolic.Readable) (*symbolic.ByteSlice, *symbolic.Error) {
			return &symbolic.ByteSlice{}, nil
		},

		encodeHex, func(ctx *symbolic.Context, arg symbolic.Readable) *symbolic.String {
			return &symbolic.String{}
		},

		decodeHex, func(ctx *symbolic.Context, arg symbolic.Readable) (*symbolic.ByteSlice, *symbolic.Error) {
			return &symbolic.ByteSlice{}, nil
		},

		_tostr, func(ctx *symbolic.Context, arg symbolic.SymbolicValue) symbolic.StringLike {
			return symbolic.ANY_STR_LIKE
		},
		_torune, func(ctx *symbolic.Context, arg symbolic.Integral) *symbolic.Rune {
			return &symbolic.Rune{}
		},
		_tobyte, func(ctx *symbolic.Context, arg *symbolic.Int) *symbolic.Byte {
			return &symbolic.Byte{}
		},
		_tofloat, func(ctx *symbolic.Context, arg *symbolic.Int) *symbolic.Float {
			return &symbolic.Float{}
		},
		_torstream, func(ctx *symbolic.Context, arg symbolic.SymbolicValue) *symbolic.ReadableStream {
			return symbolic.NewReadableStream(symbolic.ANY)
		},

		//

		core.ToJSON, func(ctx *symbolic.Context, arg symbolic.SymbolicValue) *symbolic.String {
			return &symbolic.String{}
		},
		core.ToPrettyJSON, func(ctx *symbolic.Context, arg symbolic.SymbolicValue) *symbolic.String {
			return &symbolic.String{}
		},

		_repr, func(ctx *symbolic.Context, arg symbolic.SymbolicValue) *symbolic.String {
			return &symbolic.String{}
		},
		_parse_repr, func(ctx *symbolic.Context, arg symbolic.Readable) (symbolic.SymbolicValue, *symbolic.Error) {
			return symbolic.ANY, nil
		},
		_parse, func(ctx *symbolic.Context, arg symbolic.Readable, p symbolic.Pattern) (symbolic.SymbolicValue, *symbolic.Error) {
			return p.SymbolicValue(), nil
		},
		_split, func(ctx *symbolic.Context, arg symbolic.Readable, sep *symbolic.String, p symbolic.Pattern) (symbolic.SymbolicValue, *symbolic.Error) {
			return symbolic.NewListOf(p.SymbolicValue()), nil
		},

		_idt, func(ctx *symbolic.Context, arg symbolic.SymbolicValue) symbolic.SymbolicValue {
			return arg
		},
		_len, func(ctx *symbolic.Context, arg symbolic.SymbolicValue) *symbolic.Int {
			return &symbolic.Int{}
		},
		_len_range, func(ctx *symbolic.Context, arg symbolic.StringPatternElement) *symbolic.IntRange {
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

		_dynimport, func(ctx *symbolic.Context, src symbolic.SymbolicValue, argObj *symbolic.Object, manifestObj *symbolic.Object, options ...symbolic.SymbolicValue) (*symbolic.Routine, *symbolic.Error) {
			return &symbolic.Routine{}, nil
		},
		_run, func(ctx *symbolic.Context, src *symbolic.Path, args ...symbolic.SymbolicValue) *symbolic.Error {
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
		core.Append, func(ctx *symbolic.Context, slice *symbolic.List, args ...symbolic.SymbolicValue) *symbolic.List {
			if slice.HasKnownLen() {
				//TODO: update elements
			}
			return slice
		},

		//

		core.NewEventSource, func(ctx *symbolic.Context, resourceNameOrPattern symbolic.SymbolicValue) (*symbolic.EventSource, *symbolic.Error) {
			return symbolic.NewEventSource(), nil
		},

		_cancel_exec, func(ctx *symbolic.Context) {

		},

		_url_of, func(ctx *symbolic.Context, v symbolic.SymbolicValue) *symbolic.URL {
			return &symbolic.URL{}
		},
		core.IdOf, func(ctx *symbolic.Context, v symbolic.SymbolicValue) *symbolic.Identifier {
			return &symbolic.Identifier{}
		},
		//

		core.SumOptions, func(ctx *symbolic.Context, config *symbolic.Object, options ...*symbolic.Option) (*symbolic.Object, *symbolic.Error) {
			return symbolic.NewAnyObject(), nil
		},

		_List, func(ctx *symbolic.Context, args ...symbolic.SymbolicValue) *symbolic.List {
			return symbolic.NewListOf(symbolic.ANY)
		},
		_Event, func(ctx *symbolic.Context, value symbolic.SymbolicValue) *symbolic.Event {
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
		_Color, func(ctx *symbolic.Context, firstArg symbolic.SymbolicValue, others ...symbolic.SymbolicValue) *symbolic.Color {
			return &symbolic.Color{}
		},

		_add_ctx_data, func(ctx *symbolic.Context, name *symbolic.Identifier, value symbolic.SymbolicValue) {

		},
		_ctx_data, func(ctx *symbolic.Context, name *symbolic.Identifier) symbolic.SymbolicValue {
			return symbolic.ANY
		},
		_get_system_graph, func(ctx *symbolic.Context) (*symbolic.SystemGraph, *symbolic.Bool) {
			return symbolic.ANY_SYSTEM_GRAPH, symbolic.ANY_BOOL
		},

		_propnames, func(ctx *symbolic.Context, v symbolic.SymbolicValue) *symbolic.List {
			if _, ok := v.(symbolic.IProps); !ok {
				ctx.AddSymbolicGoFunctionError("value cannot have properties")
			}
			return symbolic.NewListOf(symbolic.ANY_STR)
		},
	})

}
