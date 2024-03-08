package symbolic

import (
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/prettyprint"
	"github.com/inoxlang/inox/internal/utils"
)

var (
	ANY_DEV_API       = &DevAPI{}
	DEV_API_PROPNAMES = []string{}

	_ = symbolic.GoValue((*DevAPI)(nil))
)

type DevAPI struct {
}

func (a *DevAPI) Test(v symbolic.Value, state symbolic.RecTestCallState) bool {
	return utils.Implements[*DevAPI](v)
}

func (a *DevAPI) GetGoMethod(name string) (*symbolic.GoFunction, bool) {
	return nil, false
}

func (a *DevAPI) Prop(name string) symbolic.Value {
	switch name {

	}
	method, ok := a.GetGoMethod(name)
	if !ok {
		panic(symbolic.FormatErrPropertyDoesNotExist(name, a))
	}
	return method
}

func (a *DevAPI) PropertyNames() []string {
	return DEV_API_PROPNAMES
}

func (a *DevAPI) IsMutable() bool {
	return true
}

func (a *DevAPI) PrettyPrint(w prettyprint.PrettyPrintWriter, config *prettyprint.PrettyPrintConfig) {
	w.WriteName("dev-api")
}

func (d *DevAPI) WidestOfType() symbolic.Value {
	return ANY_DEV_API
}
