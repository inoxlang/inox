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
		_html_find, func(ctx *symbolic.Context, selector *symbolic.String, node *_html_symbolic.HTMLNode) *symbolic.List {
			return symbolic.NewListOf(_html_symbolic.NewHTMLNode())
		},
		NewNode, symbolicElement,
		Render, func(ctx *symbolic.Context, arg symbolic.Value) *symbolic.ByteSlice {
			return &symbolic.ByteSlice{}
		},
		RenderToString, func(ctx *symbolic.Context, arg symbolic.Value) *symbolic.String {
			return symbolic.ANY_STRING
		},
		EscapeString, func(ctx *symbolic.Context, s symbolic.StringLike) *symbolic.String {
			return symbolic.ANY_STRING
		},
		CreateHTMLNodeFromXMLElement, _html_symbolic.CreateHTMLNodeFromXMLElement,
	})

	specifcTagFactory := func(ctx *symbolic.Context, desc *symbolic.Object) *_html_symbolic.HTMLNode {
		return symbolicElement(ctx, symbolic.ANY_STRING, desc)
	}

	for _, fn := range []any{} {
		core.RegisterSymbolicGoFunction(fn, specifcTagFactory)
	}

	help.RegisterHelpValues(map[string]any{
		"html.Node":   NewNode,
		"html.find":   _html_find,
		"html.escape": EscapeString,
	})
}

func NewHTMLNamespace() *core.Namespace {
	return core.NewNamespace("html", map[string]core.Value{
		"find":       core.WrapGoFunction(_html_find),
		"Node":       core.WrapGoFunction(NewNode),
		"render":     core.WrapGoFunction(Render),
		"str_render": core.WrapGoFunction(RenderToString),
		"escape":     core.WrapGoFunction(EscapeString),

		symbolic.FROM_XML_FACTORY_NAME: core.WrapGoFunction(CreateHTMLNodeFromXMLElement),
	})
}
