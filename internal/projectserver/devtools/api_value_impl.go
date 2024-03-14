package devtools

import (
	"bufio"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/core/symbolicdev"
)

var _ = core.GoValue((*API)(nil))

func (a *API) GetGoMethod(name string) (*core.GoFunction, bool) {
	switch name {
	case "get_db":
		return core.WrapGoMethod(a.getDB), true
	case "get_db_names":
		return core.WrapGoMethod(a.getDatabaseNames), true
	}
	return nil, false
}

func (a *API) Prop(ctx *core.Context, name string) core.Value {
	switch name {

	}

	method, ok := a.GetGoMethod(name)
	if !ok {
		panic(core.FormatErrPropertyDoesNotExist(name, a))
	}
	return method
}

func (a *API) PropertyNames(*core.Context) []string {
	return symbolicdev.DEV_API_PROPNAMES
}

func (a *API) SetProp(ctx *core.Context, name string, value core.Value) error {
	return core.ErrCannotSetProp
}

func (a *API) Equal(ctx *core.Context, other core.Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherAPI, ok := other.(*API)
	return ok && a == otherAPI
}

func (a *API) IsMutable() bool {
	return true
}

func (a *API) PrettyPrint(w *bufio.Writer, config *core.PrettyPrintConfig, depth int, parentIndentCount int) {
	w.WriteString("dev-api")
}

func (d *API) ToSymbolicValue(ctx *core.Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return symbolicdev.ANY_DEV_API, nil
}
