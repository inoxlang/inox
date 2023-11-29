package symbolic

import (
	"fmt"

	pprint "github.com/inoxlang/inox/internal/prettyprint"
	"github.com/inoxlang/inox/internal/utils"
	"golang.org/x/exp/slices"
)

var (
	API_PROPNAMES = []string{"version", "schema", "data"}

	ANY_API = NewApiIL(NewAnyObjectPattern())
)

// A ApiIL represents a symbolic ApiIL.
type ApiIL struct {
	UnassignablePropsMixin
	schema        *ObjectPattern
	data          *Object
	propertyNames []string
}

func NewApiIL(schema *ObjectPattern) *ApiIL {
	propertyNames := slices.Clone(API_PROPNAMES)
	for propName := range schema.entries {
		if utils.SliceContains(API_PROPNAMES, propName) {
			panic(fmt.Errorf("name collision with inital property name '%s'", propName))
		}
		propertyNames = append(propertyNames, propName)
	}

	return &ApiIL{
		schema:        schema,
		data:          schema.SymbolicValue().(*Object),
		propertyNames: propertyNames,
	}
}

func (api *ApiIL) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	switch other := v.(type) {
	case *ApiIL:
		return api.schema.Test(other.schema, state)
	default:
		return false
	}
}

func (api *ApiIL) WidestOfType() Value {
	return ANY_API
}

func (api *ApiIL) GetGoMethod(name string) (*GoFunction, bool) {
	switch name {
	}
	return nil, false
}

func (api *ApiIL) Prop(name string) Value {
	switch name {
	case "version":
		return ANY_STR
	case "schema":
		return api.schema
	case "data":
		return api.data
	}

	entry, ok := api.schema.entries[name]
	if ok {
		return entry.SymbolicValue()
	}

	method, ok := api.GetGoMethod(name)
	if !ok {
		panic(FormatErrPropertyDoesNotExist(name, api))
	}
	return method
}

func (db *ApiIL) PropertyNames() []string {
	return db.propertyNames
}

func (ApiIL *ApiIL) UpdateSchema(ctx *Context, schema *ObjectPattern) *Error {
	return nil
}

func (ApiIL *ApiIL) Close(*Context) *Error {
	return nil
}

func (r *ApiIL) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("api")
}
