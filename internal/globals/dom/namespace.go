package internal

import (
	core "github.com/inox-project/inox/internal/core"
	symbolic "github.com/inox-project/inox/internal/core/symbolic"
	_dom_symbolic "github.com/inox-project/inox/internal/globals/dom/symbolic"
)

func init() {

	int_string_pattern, _ := core.INT_PATTERN.StringPattern()

	// register patterns
	core.RegisterDefaultPatternNamespace("dom", &core.PatternNamespace{
		Patterns: map[string]core.Pattern{
			"node": NODE_PATTERN,
			"click-event": core.NewEventPattern(
				core.NewInexactRecordPattern(map[string]core.Pattern{
					"type":           core.NewExactValuePattern(core.Str("click")),
					"forwarderClass": core.STR_PATTERN,
					"targetClass":    core.STR_PATTERN,
					"button":         int_string_pattern,
				}),
			),
			"keydown-event": core.NewEventPattern(
				core.NewInexactRecordPattern(map[string]core.Pattern{
					"type":           core.NewExactValuePattern(core.Str("keydown")),
					"forwarderClass": core.STR_PATTERN,
					"targetClass":    core.STR_PATTERN,

					"key":      core.STR_PATTERN,
					"ctrlKey":  core.BOOL_PATTERN,
					"altKey":   core.BOOL_PATTERN,
					"metaKey":  core.BOOL_PATTERN,
					"shiftKey": core.BOOL_PATTERN,

					//selection data
					"anchorElemData": core.RECORD_PATTERN, //TODO: replace with a record pattern with string values
					"anchorOffset":   int_string_pattern,
					"focusElemData":  core.RECORD_PATTERN, //TODO: replace with a record pattern with string values
					"focusOffset":    int_string_pattern,
				}),
			),
			"cut-event": core.NewEventPattern(
				core.NewInexactRecordPattern(map[string]core.Pattern{
					"type":           core.NewExactValuePattern(core.Str("cut")),
					"forwarderClass": core.STR_PATTERN,
					"targetClass":    core.STR_PATTERN,

					//selection data
					"anchorElemData": core.RECORD_PATTERN, //TODO: replace with a record pattern with string values
					"anchorOffset":   int_string_pattern,
					"focusElemData":  core.RECORD_PATTERN, //TODO: replace with a record pattern with string values
					"focusOffset":    int_string_pattern,
				}),
			),
			"paste-event": core.NewEventPattern(
				core.NewInexactRecordPattern(map[string]core.Pattern{
					"type":           core.NewExactValuePattern(core.Str("paste")),
					"forwarderClass": core.STR_PATTERN,
					"targetClass":    core.STR_PATTERN,

					"text": core.STR_PATTERN,

					//selection data
					"anchorElemData": core.RECORD_PATTERN, //TODO: replace with a record pattern with string values
					"anchorOffset":   int_string_pattern,
					"focusElemData":  core.RECORD_PATTERN, //TODO: replace with a record pattern with string values
					"focusOffset":    int_string_pattern,
				}),
			),
		},
	})

	symbolicElement := func(ctx *symbolic.Context, tag *symbolic.String, desc *symbolic.Object) *_dom_symbolic.Node {
		var model symbolic.SymbolicValue = symbolic.Nil
		desc.ForEachEntry(func(k string, v symbolic.SymbolicValue) error {
			switch k {
			case MODEL_KEY:
				model = v
			}
			return nil
		})
		return _dom_symbolic.NewDomNode(model)
	}

	// register symbolic version of Go functions
	core.RegisterSymbolicGoFunctions([]any{
		NewNode, symbolicElement,
		NewAutoNode, func(ctx *symbolic.Context, model symbolic.SymbolicValue, args ...symbolic.SymbolicValue) *_dom_symbolic.Node {
			if len(args) > 1 {
				ctx.AddSymbolicGoFunctionError("at most two arguments were expected")
			}
			return _dom_symbolic.NewDomNode(model)
		},
	})

	specifcTagFactory := func(ctx *symbolic.Context, desc *symbolic.Object) *_dom_symbolic.Node {
		return symbolicElement(ctx, &symbolic.String{}, desc)
	}

	for _, fn := range []any{_a, _div, _ul, _ol, _li, _span, _svg, _h1, _h2, _h3, _h4} {
		core.RegisterSymbolicGoFunction(fn, specifcTagFactory)
	}

}

func NewDomNamespace() *core.Record {
	return core.NewRecordFromMap(core.ValMap{
		"Node": core.WrapGoFunction(NewNode),
		"auto": core.WrapGoFunction(NewAutoNode),
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
		"CSP":  core.WrapGoFunction(NewCSP),
	})
}
