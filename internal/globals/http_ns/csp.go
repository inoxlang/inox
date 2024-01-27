package http_ns

import (
	"bytes"
	"fmt"
	"slices"

	"io"
	"sort"

	"github.com/inoxlang/inox/internal/commonfmt"
	"github.com/inoxlang/inox/internal/core"
	jsoniter "github.com/inoxlang/inox/internal/jsoniter"
)

const (
	CSP_HEADER_NAME          = "Content-Security-Policy"
	DEFAULT_NONCE_BYTE_COUNT = 16
)

var (
	DEFAULT_DIRECTIVE_VALUES = map[string][]CSPDirectiveValue{
		"default-src": {{raw: "'none'"}},

		"frame-ancestors": {{raw: "'none'"}},
		"frame-src":       {{raw: "'none'"}},

		"script-src-elem": {{raw: "'self'"}},
		"connect-src":     {{raw: "'self'"}},

		"font-src":       {{raw: "'self'"}},
		"img-src":        {{raw: "'self'"}},
		"style-src-elem": {{raw: "'self' 'unsafe-inline'"}},
	}

	_ = []core.Value{(*ContentSecurityPolicy)(nil)}
)

type ContentSecurityPolicy struct {
	directives map[string]CSPDirective
}

func NewCSP(ctx *core.Context, desc *core.Object) (*ContentSecurityPolicy, error) {
	var directives []CSPDirective

	for k, v := range desc.EntryMap(ctx) {
		directive := CSPDirective{name: k}

		switch directiveDesc := v.(type) {
		case core.String:
			directive.values = append(directive.values, CSPDirectiveValue{raw: string(directiveDesc)})
		case core.Iterable:
			iterable := directiveDesc
			it := iterable.Iterator(ctx, core.IteratorConfiguration{})

			for it.Next(ctx) {
				val := it.Value(ctx)
				s, ok := val.(core.String)
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
				values: slices.Clone(v),
			}
		}
	}

	return &ContentSecurityPolicy{directives: directiveMap}, nil
}

func (c *ContentSecurityPolicy) Directive(directiveName string) (CSPDirective, bool) {
	d, ok := c.directives[directiveName]
	return d, ok
}

func (c *ContentSecurityPolicy) writeToBuf(buf *bytes.Buffer, scriptElemsNonce string) {
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

		if scriptElemsNonce != "" && k == "script-src-elem" {
			buf.WriteString(" 'nonce-")
			buf.WriteString(scriptElemsNonce)
			buf.WriteByte('\'')
		}

		if i == len(keys)-1 {
			buf.WriteString(";")
		} else {
			buf.WriteString("; ")
		}
	}
}

type CSPHeaderValueParams struct {
	ScriptsNonce string //optional
}

func (csp *ContentSecurityPolicy) String() string {
	nonceExample := "<rand string such as " + randomCSPNonce() + ">"

	return csp.HeaderValue(CSPHeaderValueParams{ScriptsNonce: nonceExample})
}

func (c *ContentSecurityPolicy) HeaderValue(params CSPHeaderValueParams) string {
	buf := bytes.NewBuffer(nil)
	c.writeToBuf(buf, params.ScriptsNonce)
	return buf.String()
}

func (c *ContentSecurityPolicy) WriteRepresentation(ctx *core.Context, w io.Writer, config *core.ReprConfig, depth int) error {
	return core.ErrNotImplementedYet
}

func (c *ContentSecurityPolicy) WriteJSONRepresentation(ctx *core.Context, w *jsoniter.Stream, config core.JSONSerializationConfig, depth int) error {
	return core.ErrNotImplementedYet
}

type CSPDirective struct {
	name   string
	values []CSPDirectiveValue
}

type CSPDirectiveValue struct {
	raw string
}

// randomCSPNonce returns a random nonce value (unppaded base64), 'nonce-' is NOT part of the returned string.
func randomCSPNonce() string {
	//note: the random string should not start with the '-' character because this can cause issues.

	return core.CryptoRandSource.ReadNBytesAsBase64Unpadded(DEFAULT_NONCE_BYTE_COUNT)
}
