package symbolicdev

import (
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/prettyprint"
	"github.com/inoxlang/inox/internal/utils"
)

var (
	ANY_DEV_API       = &API{}
	DEV_API_PROPNAMES = []string{"get_db"}

	_ = symbolic.GoValue((*API)(nil))
	_ = symbolic.IProps((*API)(nil))
)

type API struct {
	symbolic.UnassignablePropsMixin
}

func (a *API) Test(v symbolic.Value, state symbolic.RecTestCallState) bool {
	return utils.Implements[*API](v)
}

func (a *API) getDB(ctx *symbolic.Context, name *symbolic.String) (*DBProxy, *symbolic.Error) {
	return ANY_DB_PROXY, nil
}

func (a *API) GetGoMethod(name string) (*symbolic.GoFunction, bool) {
	switch name {
	case "get_db":
		return symbolic.WrapGoMethod(a.getDB), true
	}
	return nil, false
}

func (a *API) Prop(name string) symbolic.Value {
	switch name {

	}
	method, ok := a.GetGoMethod(name)
	if !ok {
		panic(symbolic.FormatErrPropertyDoesNotExist(name, a))
	}
	return method
}

func (a *API) PropertyNames() []string {
	return DEV_API_PROPNAMES
}

func (a *API) IsMutable() bool {
	return true
}

func (a *API) PrettyPrint(w prettyprint.PrettyPrintWriter, config *prettyprint.PrettyPrintConfig) {
	w.WriteName("dev-api")
}

func (a *API) WidestOfType() symbolic.Value {
	return ANY_DEV_API
}
