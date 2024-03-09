package symbolicdev

import (
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/prettyprint"
	"github.com/inoxlang/inox/internal/utils"
)

var (
	ANY_DB_PROXY       = &DBProxy{}
	DB_PROXY_PROPNAMES = []string{"get_schema"}

	_ = symbolic.GoValue((*DBProxy)(nil))
	_ = symbolic.IProps((*DBProxy)(nil))
)

type DBProxy struct {
	symbolic.UnassignablePropsMixin
}

func (p *DBProxy) Test(v symbolic.Value, state symbolic.RecTestCallState) bool {
	return utils.Implements[*DBProxy](v)
}

func (p *DBProxy) GetSchema(ctx *symbolic.Context) (*symbolic.ObjectPattern, *symbolic.Error) {
	return symbolic.ANY_OBJECT_PATTERN, nil
}

func (p *DBProxy) GetGoMethod(name string) (*symbolic.GoFunction, bool) {
	switch name {
	case "get_schema":
		return symbolic.WrapGoMethod(p.GetSchema), true
	}
	return nil, false
}

func (p *DBProxy) Prop(name string) symbolic.Value {
	switch name {

	}
	method, ok := p.GetGoMethod(name)
	if !ok {
		panic(symbolic.FormatErrPropertyDoesNotExist(name, p))
	}
	return method
}

func (p *DBProxy) PropertyNames() []string {
	return DB_PROXY_PROPNAMES
}

func (p *DBProxy) IsMutable() bool {
	return true
}

func (p *DBProxy) PrettyPrint(w prettyprint.PrettyPrintWriter, config *prettyprint.PrettyPrintConfig) {
	w.WriteName("db-proxy")
}

func (p *DBProxy) WidestOfType() symbolic.Value {
	return ANY_DB_PROXY
}
