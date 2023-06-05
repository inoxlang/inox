package sql_ns

import core "github.com/inoxlang/inox/internal/core"

//postgres

var (
	_ = []core.StatelessParser{&sqlQueryParser{}}
)

type sqlQueryParser struct {
	underlying *underlyingQueryParser
}

func newQueryParser() *sqlQueryParser {
	p := &sqlQueryParser{
		newUnderlyingQueryParser(),
	}

	return p
}

// Validate checks that the string is a valid SQL statement with both MySQL & Postgres parsers.
func (p *sqlQueryParser) Validate(ctx *core.Context, s string) bool {
	return p.underlying.Validate(ctx, s)
}

func (p *sqlQueryParser) Parse(ctx *core.Context, s string) (core.Value, error) {
	return nil, core.ErrNotImplementedYet
}

type sqlStringValueParser struct {
	underlying *underlyingStringValueParser
}

func newStringValueParser() *sqlStringValueParser {
	p := &sqlStringValueParser{
		newUnderlyingStringValueParser(),
	}

	return p
}

// Validate checks that the string is a valid SQL string literal with the MySQL parser.
func (p *sqlStringValueParser) Validate(ctx *core.Context, s string) bool {
	return p.underlying.Validate(ctx, s)
}

func (p *sqlStringValueParser) Parse(ctx *core.Context, s string) (core.Value, error) {
	return nil, core.ErrNotImplementedYet
}
