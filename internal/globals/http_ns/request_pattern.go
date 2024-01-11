package http_ns

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/inoxlang/inox/internal/commonfmt"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/symbolic"

	"github.com/inoxlang/inox/internal/globals/http_ns/spec"
	http_symbolic "github.com/inoxlang/inox/internal/globals/http_ns/symbolic"

	"slices"

	"github.com/inoxlang/inox/internal/utils"
)

var (
	_ core.Pattern = (*HttpRequestPattern)(nil)

	CALLABLE_HTTP_REQUEST_PATTERN = &core.TypePattern{
		Name:          "http.req",
		Type:          reflect.TypeOf(&HttpRequest{}),
		SymbolicValue: &http_symbolic.HttpRequest{},
		CallImpl: func(callee *core.TypePattern, values []core.Serializable) (core.Pattern, error) {
			if len(values) != 1 {
				return nil, errors.New("a single argument is supported for now, it should be an object pattern")
			}
			objPattern, ok := values[0].(*core.ObjectPattern)
			if !ok {
				return nil, errors.New("argument should be an object pattern")
			}

			//if nil, any method is matched.
			var methods []string

			err := objPattern.ForEachEntry(func(propName string, propPattern core.Pattern, isOptional bool) error {
				switch propName {
				case "method":
					switch p := propPattern.(type) {
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
			return &HttpRequestPattern{
				methods: methods,
				headers: core.NewInexactRecordPattern(nil),
				CallBasedPatternReprMixin: core.CallBasedPatternReprMixin{
					Callee: callee,
					Params: values,
				},
			}, nil
		},
		SymbolicCallImpl: func(ctx *symbolic.Context, values []symbolic.Value) (symbolic.Pattern, error) {
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

			return &http_symbolic.HttpRequestPattern{}, nil
		},
	}
)

type HttpRequestPattern struct {
	methods []string //if nil any method is accepted
	headers *core.RecordPattern

	core.CallBasedPatternReprMixin
	core.NotCallablePatternMixin
}

func (p *HttpRequestPattern) Test(ctx *core.Context, v core.Value) bool {
	req, ok := v.(*HttpRequest)

	if !ok || (p.methods != nil && !utils.SliceContains(p.methods, req.Method.UnderlyingString())) {
		return false
	}

	if !p.headers.Test(ctx, req.headers) {
		return false
	}

	return true
}

func (*HttpRequestPattern) Iterator(*core.Context, core.IteratorConfiguration) core.Iterator {
	return core.NewEmptyPatternIterator()
}

func (*HttpRequestPattern) Random(ctx *core.Context, options ...core.Option) core.Value {
	panic(core.ErrNotImplemented)
}

func (*HttpRequestPattern) StringPattern() (core.StringPattern, bool) {
	return nil, false
}
