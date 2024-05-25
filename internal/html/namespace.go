package html_ns

import (
	"github.com/inoxlang/inox/internal/core"

	"github.com/inoxlang/inox/internal/core/symbolic"
	_html_symbolic "github.com/inoxlang/inox/internal/html/symbolic"
)

func init() {

	// register patterns
	core.RegisterDefaultPatternNamespace("html", &core.PatternNamespace{
		Patterns: map[string]core.Pattern{
			"node": NODE_PATTERN,
		},
	})

	symbolicElement := func(ctx *symbolic.Context, tag *symbolic.String, desc *symbolic.Object) *_html_symbolic.HTMLNode {
		return _html_symbolic.ANY_HTML_NODE
	}

	// register symbolic version of Go functions
	core.RegisterSymbolicGoFunctions([]any{
		_html_find, func(ctx *symbolic.Context, selector *symbolic.String, node *_html_symbolic.HTMLNode) *symbolic.List {
			return symbolic.NewListOf(_html_symbolic.ANY_HTML_NODE)
		},
		NewNode, symbolicElement,
		Render, func(ctx *symbolic.Context, arg *_html_symbolic.HTMLNode) *symbolic.ByteSlice {
			return symbolic.ANY_BYTE_SLICE
		},
		RenderToString, func(ctx *symbolic.Context, arg *_html_symbolic.HTMLNode) *symbolic.String {
			return symbolic.ANY_STRING
		},
		EscapeString, func(ctx *symbolic.Context, s symbolic.StringLike) *symbolic.String {
			return symbolic.ANY_STRING
		},
		UnescapeString, func(ctx *symbolic.Context, s symbolic.StringLike) *symbolic.String {
			return symbolic.ANY_STRING
		},
		CreateHTMLNodeFromMarkupElement, _html_symbolic.CreateHTMLNodeFromMarkupElement,
	})

	specifcTagFactory := func(ctx *symbolic.Context, desc *symbolic.Object) *_html_symbolic.HTMLNode {
		return symbolicElement(ctx, symbolic.ANY_STRING, desc)
	}

	for _, fn := range []any{} {
		core.RegisterSymbolicGoFunction(fn, specifcTagFactory)
	}

	// help.RegisterHelpValues(map[string]any{
	// 	"html.Node":   NewNode,
	// 	"html.find":   _html_find,
	// 	"html.escape": EscapeString,
	// })
}

func NewHTMLNamespace() *core.Namespace {
	return core.NewNamespace("html", map[string]core.Value{
		"find":       core.WrapGoFunction(_html_find),
		"Node":       core.WrapGoFunction(NewNode),
		"render":     core.WrapGoFunction(Render),
		"str_render": core.WrapGoFunction(RenderToString),
		"escape":     core.WrapGoFunction(EscapeString),
		"unespace":   core.WrapGoFunction(UnescapeString),

		symbolic.FROM_MARKUP_FACTORY_NAME: core.WrapGoFunction(CreateHTMLNodeFromMarkupElement),
	})
}
