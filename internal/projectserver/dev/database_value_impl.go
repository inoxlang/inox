package dev

import (
	"bufio"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/core/symbolicdev"
)

var _ = core.GoValue((*dbProxy)(nil))

func (p *dbProxy) GetGoMethod(name string) (*core.GoFunction, bool) {
	switch name {
	case "get_schema":
		return core.WrapGoMethod(p.getSchema), true
	}
	return nil, false
}

func (p *dbProxy) Prop(ctx *core.Context, name string) core.Value {
	switch name {
	}

	method, ok := p.GetGoMethod(name)
	if !ok {
		panic(core.FormatErrPropertyDoesNotExist(name, p))
	}
	return method
}

func (p *dbProxy) PropertyNames(*core.Context) []string {
	return symbolicdev.DB_PROXY_PROPNAMES
}

func (p *dbProxy) SetProp(ctx *core.Context, name string, value core.Value) error {
	return core.ErrCannotSetProp
}

func (p *dbProxy) Equal(ctx *core.Context, other core.Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherProxy, ok := other.(*dbProxy)
	return ok && p == otherProxy
}

func (p *dbProxy) IsMutable() bool {
	return true
}

func (p *dbProxy) PrettyPrint(w *bufio.Writer, config *core.PrettyPrintConfig, depth int, parentIndentCount int) {
	w.WriteString("db-proxy")
}

func (p *dbProxy) ToSymbolicValue(ctx *core.Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return symbolicdev.ANY_DB_PROXY, nil
}
