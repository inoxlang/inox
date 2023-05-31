//go:build js

package sql_ns

import (
	core "github.com/inoxlang/inox/internal/core"
)

var (
	_ = []core.StatelessParser{&sqlQueryParser{}}
)

type underlyingQueryParser struct {
}

func newUnderlyingQueryParser() *underlyingQueryParser {
	panic(core.ErrNotImplementedYet)
}

func (p *underlyingQueryParser) Validate(ctx *core.Context, s string) bool {
	panic(core.ErrNotImplementedYet)
}

func (p *underlyingQueryParser) Parse(ctx *core.Context, s string) (core.Value, error) {
	panic(core.ErrNotImplementedYet)
}

type underlyingStringValueParser struct {
}

func newUnderlyingStringValueParser() *underlyingStringValueParser {
	panic(core.ErrNotImplementedYet)
}

func (p *underlyingStringValueParser) Validate(ctx *core.Context, s string) bool {
	panic(core.ErrNotImplementedYet)
}

func (p *underlyingStringValueParser) Parse(ctx *core.Context, s string) (core.Value, error) {
	return nil, core.ErrNotImplementedYet
}
