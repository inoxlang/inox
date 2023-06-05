package dom_ns

import core "github.com/inoxlang/inox/internal/core"

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
