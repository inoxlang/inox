package core

import "github.com/inoxlang/inox/internal/core/symbolic"

type API interface {
	Version() string
	Schema() *ObjectPattern
	Data() *Object
}

type ApiIL struct {
	inner         API
	initialSchema *ObjectPattern
	data          *Object

	NoReprMixin
	NotClonableMixin
}

func WrapAPI(inner API) *ApiIL {
	schema := inner.Schema()

	return &ApiIL{
		inner:         inner,
		initialSchema: schema,
		data:          inner.Data(),
	}
}

func (api *ApiIL) GetGoMethod(name string) (*GoFunction, bool) {
	switch name {
	}
	return nil, false
}

func (api *ApiIL) Prop(ctx *Context, name string) Value {
	switch name {
	case "version":
		return Str(api.inner.Version())
	case "schema":
		return api.initialSchema
	case "data":
		return api.data
	}

	method, ok := api.GetGoMethod(name)
	if !ok {
		panic(FormatErrPropertyDoesNotExist(name, api))
	}
	return method
}

func (*ApiIL) SetProp(ctx *Context, name string, value Value) error {
	return ErrCannotSetProp
}

func (api *ApiIL) PropertyNames(ctx *Context) []string {
	return symbolic.API_PROPNAMES
}
