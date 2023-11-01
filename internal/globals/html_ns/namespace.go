package html_ns

import (
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/help"

	"github.com/inoxlang/inox/internal/core/symbolic"
	_html_symbolic "github.com/inoxlang/inox/internal/globals/html_ns/symbolic"
)

func init() {

	// register patterns
	core.RegisterDefaultPatternNamespace("html", &core.PatternNamespace{
		Patterns: map[string]core.Pattern{
			"node": NODE_PATTERN,
		},
	})

	symbolicElement := func(ctx *symbolic.Context, tag *symbolic.String, desc *symbolic.Object) *_html_symbolic.HTMLNode {
		return _html_symbolic.NewHTMLNode()
	}

	// register symbolic version of Go functions
	core.RegisterSymbolicGoFunctions([]any{
		_html_find, func(ctx *symbolic.Context, selector *symbolic.String, node symbolic.Value) *symbolic.List {
			return symbolic.NewListOf(_html_symbolic.NewHTMLNode())
		},
		NewNode, symbolicElement,
		Render, func(ctx *symbolic.Context, arg symbolic.Value) *symbolic.ByteSlice {
			return &symbolic.ByteSlice{}
		},
		RenderToString, func(ctx *symbolic.Context, arg symbolic.Value) *symbolic.String {
			return &symbolic.String{}
		},
		EscapeString, func(ctx *symbolic.Context, s symbolic.StringLike) *symbolic.String {
			return symbolic.ANY_STR
		},
		CreateHTMLNodeFromXMLElement, _html_symbolic.CreateHTMLNodeFromXMLElement,
	})

	specifcTagFactory := func(ctx *symbolic.Context, desc *symbolic.Object) *_html_symbolic.HTMLNode {
		return symbolicElement(ctx, &symbolic.String{}, desc)
	}

	for _, fn := range []any{_a, _div, _ul, _ol, _li, _span, _svg, _h1, _h2, _h3, _h4} {
		core.RegisterSymbolicGoFunction(fn, specifcTagFactory)
	}

	help.RegisterHelpValues(map[string]any{
		"html.h1": _h1,
		"html.h2": _h2,
		"html.h3": _h3,
		"html.h4": _h4,
	})
}

func NewHTMLNamespace() *core.Namespace {
	return core.NewNamespace("html", map[string]core.Value{
		"find": core.WrapGoFunction(_html_find),

		"Node": core.WrapGoFunction(NewNode),
		"a":    core.WrapGoFunction(_a),
		"div":  core.WrapGoFunction(_div),
		"span": core.WrapGoFunction(_span),
		"ul":   core.WrapGoFunction(_ul),
		"ol":   core.WrapGoFunction(_ol),
		"li":   core.WrapGoFunction(_li),
		"svg":  core.WrapGoFunction(_svg),
		"h1":   core.WrapGoFunction(_h1),
		"h2":   core.WrapGoFunction(_h2),
		"h3":   core.WrapGoFunction(_h3),
		"h4":   core.WrapGoFunction(_h4),

		"render":     core.WrapGoFunction(Render),
		"str_render": core.WrapGoFunction(RenderToString),
		"escape":     core.WrapGoFunction(EscapeString),

		symbolic.FROM_XML_FACTORY_NAME: core.WrapGoFunction(CreateHTMLNodeFromXMLElement),
	})
}
