package internal

import (
	core "github.com/inox-project/inox/internal/core"
	symbolic "github.com/inox-project/inox/internal/core/symbolic"
	_html_symbolic "github.com/inox-project/inox/internal/globals/html/symbolic"
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
		_html_find, func(ctx *symbolic.Context, selector *symbolic.String, node symbolic.SymbolicValue) *symbolic.List {
			return symbolic.NewListOf(_html_symbolic.NewHTMLNode())
		},
		NewNode, symbolicElement,
		Render, func(ctx *symbolic.Context, arg symbolic.SymbolicValue) *symbolic.ByteSlice {
			return &symbolic.ByteSlice{}
		},
		RenderToString, func(ctx *symbolic.Context, arg symbolic.SymbolicValue) *symbolic.String {
			return &symbolic.String{}
		},
	})

	specifcTagFactory := func(ctx *symbolic.Context, desc *symbolic.Object) *_html_symbolic.HTMLNode {
		return symbolicElement(ctx, &symbolic.String{}, desc)
	}

	for _, fn := range []any{_a, _div, _ul, _ol, _li, _span, _svg, _h1, _h2, _h3, _h4} {
		core.RegisterSymbolicGoFunction(fn, specifcTagFactory)
	}

	registerHelp()
}

func NewHTMLNamespace() *core.Record {
	return core.NewRecordFromMap(core.ValMap{
		"find": core.ValOf(_html_find),

		"Node": core.ValOf(NewNode),
		"a":    core.ValOf(_a),
		"div":  core.ValOf(_div),
		"span": core.ValOf(_span),
		"ul":   core.ValOf(_ul),
		"ol":   core.ValOf(_ol),
		"li":   core.ValOf(_li),
		"svg":  core.ValOf(_svg),
		"h1":   core.ValOf(_h1),
		"h2":   core.ValOf(_h2),
		"h3":   core.ValOf(_h3),
		"h4":   core.ValOf(_h4),

		"render":     core.ValOf(Render),
		"str_render": core.ValOf(RenderToString),
	})
}
