package http_ns

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/inoxlang/inox/internal/commonfmt"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/symbolic"
	jsoniter "github.com/inoxlang/inox/internal/jsoniter"

	"github.com/inoxlang/inox/internal/globals/http_ns/spec"
	http_symbolic "github.com/inoxlang/inox/internal/globals/http_ns/symbolic"

	"slices"

	"github.com/inoxlang/inox/internal/utils"
)

var (
	_ core.Pattern = (*RequestPattern)(nil)

	CALLABLE_HTTP_REQUEST_PATTERN = &core.TypePattern{
		Name:             "http.req",
		Type:             reflect.TypeOf(&Request{}),
		SymbolicValue:    &http_symbolic.Request{},
		CallImpl:         createRequestPattern,
		SymbolicCallImpl: createSymbolicRequestPattern,
	}

	HTTP_REQUEST_PATTERN_PATTERN = &core.TypePattern{
		Name:          "http.req-pattern",
		Type:          reflect.TypeOf(&RequestPattern{}),
		SymbolicValue: http_symbolic.ANY_REQUEST_PATTERN,
	}
)

type RequestPattern struct {
	methods []string //if nil any method is accepted
	headers *core.RecordPattern

	core.NotCallablePatternMixin
}

func createRequestPattern(ctx *core.Context, callee *core.TypePattern, values []core.Serializable) (core.Pattern, error) {
	if len(values) != 1 {
		return nil, errors.New("a single argument is supported for now, it should be an object pattern")
	}
	objPattern, ok := values[0].(*core.ObjectPattern)
	if !ok {
		return nil, errors.New("argument should be an object pattern")
	}

	//if nil, any method is matched.
	var methods []string

	err := objPattern.ForEachEntry(func(entry core.ObjectPatternEntry) error {
		switch entry.Name {
		case "method":
			switch p := entry.Pattern.(type) {
			case *core.ExactValuePattern:
				methods = []string{p.Value().(core.Identifier).UnderlyingString()}
			case *core.UnionPattern:
				for _, case_ := range p.Cases() {
					methods = []string{case_.(*core.ExactValuePattern).Value().(core.Identifier).UnderlyingString()}
				}
			default:
				panic(core.ErrUnreachable)
			}
		default:
			panic(core.ErrUnreachable)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &RequestPattern{
		methods: methods,
		headers: core.NewInexactRecordPattern(nil),
	}, nil
}

func createSymbolicRequestPattern(ctx *symbolic.Context, values []symbolic.Value) (symbolic.Pattern, error) {
	const OBJ_ARG_NAME = "description"

	if len(values) != 1 {
		return nil, errors.New("a single argument is supported for now, it should be an object pattern")
	}
	objPattern, ok := values[0].(*symbolic.ObjectPattern)
	if !ok {
		return nil, errors.New("argument should be an object pattern")
	}
	err := objPattern.ForEachEntry(func(propName string, propPattern symbolic.Pattern, isOptional bool) error {
		switch propName {
		case "method":
			var ERR_MSG = fmt.Sprintf("either an identifier with a value among (%s) or a union is expected", strings.Join(spec.METHODS, ", "))

			checkMethod := func(i *symbolic.Identifier) bool {
				return i.HasConcreteName() && slices.Contains(spec.METHODS, i.Name())
			}

			switch p := propPattern.(type) {
			case *symbolic.ExactValuePattern:
				ident, ok := p.GetVal().(*symbolic.Identifier)
				if !ok || !checkMethod(ident) {
					return commonfmt.FmtInvalidValueForPropXOfArgY(propName, OBJ_ARG_NAME, ERR_MSG)
				}
			case *symbolic.UnionPattern:
				for _, case_ := range p.Cases() {
					exactValuePatt, ok := case_.(*symbolic.ExactValuePattern)
					if !ok {
						return commonfmt.FmtInvalidValueForPropXOfArgY(propName, OBJ_ARG_NAME, ERR_MSG)
					}
					ident, ok := exactValuePatt.GetVal().(*symbolic.Identifier)
					if !ok || !checkMethod(ident) {
						return commonfmt.FmtInvalidValueForPropXOfArgY(propName, OBJ_ARG_NAME, ERR_MSG)
					}
				}
			}
		default:
			return commonfmt.FmtUnexpectedPropInArgX(propName, OBJ_ARG_NAME)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return &http_symbolic.RequestPattern{}, nil
}

func (p *RequestPattern) Test(ctx *core.Context, v core.Value) bool {
	req, ok := v.(*Request)

	if !ok || (p.methods != nil && !utils.SliceContains(p.methods, req.Method.UnderlyingString())) {
		return false
	}

	if !p.headers.Test(ctx, req.headers) {
		return false
	}

	return true
}

func (*RequestPattern) Iterator(*core.Context, core.IteratorConfiguration) core.Iterator {
	return core.NewEmptyPatternIterator()
}

func (*RequestPattern) Random(ctx *core.Context, options ...core.Option) core.Value {
	panic(core.ErrNotImplemented)
}

func (*RequestPattern) StringPattern() (core.StringPattern, bool) {
	return nil, false
}

func (p *RequestPattern) WriteJSONRepresentation(ctx *core.Context, w *jsoniter.Stream, config core.JSONSerializationConfig, depth int) error {
	if depth > core.MAX_JSON_REPR_WRITING_DEPTH {
		return core.ErrMaximumJSONReprWritingDepthReached
	}

	return core.ErrNotImplementedYet
}

func DeserializeHttpRequestPattern(ctx *core.Context, it *jsoniter.Iterator, pattern core.Pattern, try bool) (_ core.Pattern, finalErr error) {
	panic(core.ErrNotImplementedYet)
}
