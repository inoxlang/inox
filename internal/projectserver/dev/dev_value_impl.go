package dev

import (
	"bufio"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/symbolic"
	devsymbolic "github.com/inoxlang/inox/internal/projectserver/dev/symbolic"
)

var _ = core.GoValue((*DevAPI)(nil))

func (a *DevAPI) GetGoMethod(name string) (*core.GoFunction, bool) {
	switch name {
	}
	return nil, false
}

func (a *DevAPI) Prop(ctx *core.Context, name string) core.Value {
	switch name {

	}

	method, ok := a.GetGoMethod(name)
	if !ok {
		panic(core.FormatErrPropertyDoesNotExist(name, a))
	}
	return method
}

func (a *DevAPI) PropertyNames(*core.Context) []string {
	return devsymbolic.DEV_API_PROPNAMES
}

func (a *DevAPI) SetProp(ctx *core.Context, name string, value core.Value) error {
	return core.ErrCannotSetProp
}

func (a *DevAPI) Equal(ctx *core.Context, other core.Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherAPI, ok := other.(*DevAPI)
	return ok && a == otherAPI
}

func (a *DevAPI) IsMutable() bool {
	return true
}

func (a *DevAPI) PrettyPrint(w *bufio.Writer, config *core.PrettyPrintConfig, depth int, parentIndentCount int) {
	w.WriteString("dev-api")
}

func (d *DevAPI) ToSymbolicValue(ctx *core.Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return devsymbolic.ANY_DEV_API, nil
}
