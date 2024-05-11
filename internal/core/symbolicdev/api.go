package symbolicdev

import (
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/prettyprint"
	"github.com/inoxlang/inox/internal/utils"
)

var (
	ANY_DEV_API       = &API{}
	DEV_API_PROPNAMES = []string{"get_db", "get_db_names", "get_components"}
	COMPONENT         = symbolic.NewInexactObject2(map[string]symbolic.Serializable{
		"name": symbolic.ANY_STRING,
		"position": symbolic.NewInexactRecord(map[string]symbolic.Serializable{
			"source": symbolic.ANY_STRING,
			"line":   symbolic.ANY_INT,
			"column": symbolic.ANY_INT,
			"start":  symbolic.ANY_INT,
			"end":    symbolic.ANY_INT,
		}, nil),
	})

	COMPONENT_LIST = symbolic.NewListOf(COMPONENT)

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

func (a *API) getDatabaseNames(ctx *symbolic.Context) *symbolic.List {
	return symbolic.NewListOf(symbolic.ANY_STR_LIKE)
}

func (a *API) getComponents(ctx *symbolic.Context) *symbolic.List {
	return COMPONENT_LIST
}

func (a *API) GetGoMethod(name string) (*symbolic.GoFunction, bool) {
	switch name {
	case "get_db":
		return symbolic.WrapGoMethod(a.getDB), true
	case "get_db_names":
		return symbolic.WrapGoMethod(a.getDatabaseNames), true
	case "get_components":
		return symbolic.WrapGoMethod(a.getComponents), true
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
