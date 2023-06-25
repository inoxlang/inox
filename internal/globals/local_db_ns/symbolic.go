package local_db_ns

import (
	"bufio"

	core "github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/symbolic"
	pprint "github.com/inoxlang/inox/internal/pretty_print"

	"github.com/inoxlang/inox/internal/utils"
)

//

type SymbolicLocalDatabase struct {
	symbolic.UnassignablePropsMixin
	_ int
}

func (ldb *SymbolicLocalDatabase) Test(v SymbolicValue) bool {
	_, ok := v.(*SymbolicLocalDatabase)
	return ok
}

func (r SymbolicLocalDatabase) Clone(clones map[uintptr]SymbolicValue) symbolic.SymbolicValue {
	return &SymbolicLocalDatabase{}
}

func (ldb *SymbolicLocalDatabase) Widen() (symbolic.SymbolicValue, bool) {
	return nil, false
}

func (ldb *SymbolicLocalDatabase) UpdateSchema(ctx *symbolic.Context, schema *symbolic.ObjectPattern) {

}

func (ldb *SymbolicLocalDatabase) Close() {

}

func (ldb *SymbolicLocalDatabase) GetFullResourceName(pth Path) symbolic.ResourceName {
	return &symbolic.AnyResourceName{}
}

func (ldb *SymbolicLocalDatabase) Prop(name string) SymbolicValue {
	method, ok := ldb.GetGoMethod(name)
	if !ok {
		panic(symbolic.FormatErrPropertyDoesNotExist(name, ldb))
	}
	return method
}

func (ldb *SymbolicLocalDatabase) GetGoMethod(name string) (*symbolic.GoFunction, bool) {
	switch name {
	case "update_schema":
		return symbolic.WrapGoMethod(ldb.UpdateSchema), true
	case "close":
		return symbolic.WrapGoMethod(ldb.Close), true
	}
	return nil, false
}

func (ldb *SymbolicLocalDatabase) PropertyNames() []string {
	return LOCAL_DB_PROPNAMES
}

func (ldb *SymbolicLocalDatabase) IsWidenable() bool {
	return false
}

func (ldb *SymbolicLocalDatabase) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%local-database")))
}

func (ldb *SymbolicLocalDatabase) WidestOfType() SymbolicValue {
	return &SymbolicLocalDatabase{}
}

///

func (ldb *LocalDatabase) ToSymbolicValue(ctx *core.Context, encountered map[uintptr]SymbolicValue) (SymbolicValue, error) {
	return &SymbolicLocalDatabase{}, nil
}
