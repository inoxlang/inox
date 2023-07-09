package dom_ns

import (
	"bytes"
	"fmt"

	"io"
	"sort"

	"github.com/inoxlang/inox/internal/commonfmt"
	core "github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/symbolic"
	_dom_symbolic "github.com/inoxlang/inox/internal/globals/dom_ns/symbolic"
	jsoniter "github.com/json-iterator/go"

	"github.com/inoxlang/inox/internal/utils"
)

const (
	CSP_HEADER_NAME = "Content-Security-Policy"
)

var (
	DEFAULT_DIRECTIVE_VALUES = map[string][]CSPDirectiveValue{
		"default-src": {{raw: "'none'"}},

		"frame-ancestors": {{raw: "'none'"}},
		"frame-src":       {{raw: "'none'"}},

		"script-src-elem": {{raw: "'self'"}},
		"connect-src":     {{raw: "'self'"}},

		"font-src":  {{raw: "'self'"}},
		"img-src":   {{raw: "'self'"}},
		"style-src": {{raw: "'self'"}},
	}

	_ = []core.Value{&ContentSecurityPolicy{}}
)

func init() {

	stringOrStringList := symbolic.NewMultivalue(
		symbolic.NewListOf(symbolic.ANY_STR_LIKE),
		symbolic.ANY_STR_LIKE,
	)

	core.RegisterSymbolicGoFunction(NewCSP,
		func(ctx *symbolic.Context, desc *symbolic.Object) (*_dom_symbolic.ContentSecurityPolicy, *symbolic.Error) {
			ctx.SetSymbolicGoFunctionParameters(&[]symbolic.SymbolicValue{
				symbolic.NewObject(map[string]symbolic.SymbolicValue{
					"default-src":     stringOrStringList,
					"frame-ancestors": stringOrStringList,
					"frame-src":       stringOrStringList,
					"script-src-elem": stringOrStringList,
					"connect-src":     stringOrStringList,
					"font-src":        stringOrStringList,
					"img-src":         stringOrStringList,
					"style-src":       stringOrStringList,
				}, map[string]struct{}{
					"default-src":     {},
					"frame-ancestors": {},
					"frame-src":       {},
					"script-src-elem": {},
					"connect-src":     {},
					"font-src":        {},
					"img-src":         {},
					"style-src":       {},
				}, nil),
			}, []string{"csp"})

			return _dom_symbolic.NewCSP(), nil
		},
	)
}

type ContentSecurityPolicy struct {
	core.NotClonableMixin

	directives map[string]CSPDirective
}

func NewCSP(ctx *core.Context, desc *core.Object) (*ContentSecurityPolicy, error) {
	var directives []CSPDirective

	for k, v := range desc.EntryMap() {
		directive := CSPDirective{name: k}

		switch directiveDesc := v.(type) {
		case core.Str:
			directive.values = append(directive.values, CSPDirectiveValue{raw: string(directiveDesc)})
		case core.Iterable:
			iterable := directiveDesc
			it := iterable.Iterator(ctx, core.IteratorConfiguration{})

			for it.Next(ctx) {
				val := it.Value(ctx)
				s, ok := val.(core.Str)
				if !ok {
					return nil, commonfmt.FmtUnexpectedElementInPropIterableOfArgX(k, "description", core.Stringify(s, ctx))
				}
				directive.values = append(directive.values, CSPDirectiveValue{raw: string(s)})
			}
		default:
			return nil, core.FmtPropOfArgXShouldBeOfTypeY(k, "description", "iterable or string", v)
		}

		directives = append(directives, directive)

	}

	return NewCSPWithDirectives(directives)
}

// NewCSPWithDirectives creates a CSP with the default directives and a list of given directives.
func NewCSPWithDirectives(directives []CSPDirective) (*ContentSecurityPolicy, error) {
	directiveMap := make(map[string]CSPDirective, len(directives))

	for _, d := range directives {
		if _, ok := directiveMap[d.name]; ok {
			return nil, fmt.Errorf("directive '%s' specified at least twice", d.name)
		}
		directiveMap[d.name] = d
	}

	for k, v := range DEFAULT_DIRECTIVE_VALUES {
		if _, ok := directiveMap[k]; !ok {
			directiveMap[k] = CSPDirective{
				name:   k,
				values: utils.CopySlice(v),
			}
		}
	}

	return &ContentSecurityPolicy{directives: directiveMap}, nil
}

func (c *ContentSecurityPolicy) Directive(directiveName string) (CSPDirective, bool) {
	d, ok := c.directives[directiveName]
	return d, ok
}

func (c *ContentSecurityPolicy) Write(w io.Writer) (int, error) {
	buf := bytes.NewBuffer(nil)
	c.writeToBuf(buf)
	n, err := buf.WriteTo(w)
	return int(n), err
}

func (c *ContentSecurityPolicy) writeToBuf(buf *bytes.Buffer) {
	keys := make([]string, len(c.directives))
	i := 0
	for k := range c.directives {
		keys[i] = k
		i++
	}
	sort.Strings(keys)

	for i, k := range keys {
		buf.WriteString(k)
		for _, v := range c.directives[k].values {
			buf.WriteString(" ")
			buf.WriteString(v.raw)
		}
		if i == len(keys)-1 {
			buf.WriteString(";")
		} else {
			buf.WriteString("; ")
		}
	}
}

func (c *ContentSecurityPolicy) String() string {
	buf := bytes.NewBuffer(nil)
	c.writeToBuf(buf)
	return buf.String()
}

func (c *ContentSecurityPolicy) WriteRepresentation(ctx *core.Context, w io.Writer, config *core.ReprConfig) error {
	return core.ErrNotImplementedYet
}

func (c *ContentSecurityPolicy) WriteJSONRepresentation(ctx *core.Context, w *jsoniter.Stream, config core.JSONSerializationConfig) error {
	return core.ErrNotImplementedYet
}

type CSPDirective struct {
	name   string
	values []CSPDirectiveValue
}

type CSPDirectiveValue struct {
	raw string
}
