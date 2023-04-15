package internal

import (
	core "github.com/inoxlang/inox/internal/core"

	_ "github.com/pingcap/tidb/parser/test_driver"
)

func init() {
	core.RegisterDefaultPatternNamespace("sql", &core.PatternNamespace{
		Patterns: map[string]core.Pattern{
			"query":  core.NewParserBasePattern(newQueryParser()),
			"int":    core.NewIntRangeStringPattern(-999999999, 999999999, nil),          // TODO: not true ranges, add support for any int range
			"bigint": core.NewIntRangeStringPattern(-999999999999999999, 999999999, nil), // TODO: same
			"str":    core.NewParserBasePattern(newStringValueParser()),
		},
	})
}

func NewSQLNamespace() *core.Record {
	return core.NewRecordFromMap(core.ValMap{})
}
