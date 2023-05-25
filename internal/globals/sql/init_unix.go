//go:build unix

package internal

import core "github.com/inoxlang/inox/internal/core"

func targetSpecificInit() {
	core.RegisterDefaultPatternNamespace("sql", &core.PatternNamespace{
		Patterns: map[string]core.Pattern{
			"query":  core.NewParserBasePattern(newQueryParser()),
			"int":    core.NewIntRangeStringPattern(-999999999, 999999999, nil),          // TODO: not true ranges, add support for any int range
			"bigint": core.NewIntRangeStringPattern(-999999999999999999, 999999999, nil), // TODO: same
			"str":    core.NewParserBasePattern(newStringValueParser()),
		},
	})
}
