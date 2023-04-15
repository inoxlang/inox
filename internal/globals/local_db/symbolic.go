package internal

import (
	symbolic "github.com/inoxlang/inox/internal/core/symbolic"
)

//

type SymbolicLocalDatabase struct {
	symbolic.UnassignablePropsMixin
	_ int
}

func (r *SymbolicLocalDatabase) Test(v SymbolicValue) bool {
	_, ok := v.(*SymbolicLocalDatabase)
	return ok
}

func (r SymbolicLocalDatabase) Clone(clones map[uintptr]SymbolicValue) symbolic.SymbolicValue {
	return &SymbolicLocalDatabase{}
}

func (r *SymbolicLocalDatabase) Widen() (symbolic.SymbolicValue, bool) {
	return nil, false
}

func (ldb *SymbolicLocalDatabase) Close() {

}

func (ldb *SymbolicLocalDatabase) Get(ctx *symbolic.Context, key *symbolic.Path) (SymbolicValue, *symbolic.Bool) {
	return &symbolic.Any{}, nil
}

func (ldb *SymbolicLocalDatabase) Has(ctx *symbolic.Context, key *symbolic.Path) *symbolic.Bool {
	return &symbolic.Bool{}
}

func (ldb *SymbolicLocalDatabase) Set(ctx *symbolic.Context, key *symbolic.Path, value SymbolicValue) {

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
	case "get":
		return symbolic.WrapGoMethod(ldb.Get), true
	case "has":
		return symbolic.WrapGoMethod(ldb.Has), true
	case "set":
		return symbolic.WrapGoMethod(ldb.Set), true
	case "close":
		return symbolic.WrapGoMethod(ldb.Close), true
	}
	return nil, false
}

func (ldb *SymbolicLocalDatabase) PropertyNames() []string {
	return []string{"get", "has", "set", "close"}
}

func (a *SymbolicLocalDatabase) IsWidenable() bool {
	return false
}

func (r *SymbolicLocalDatabase) String() string {
	return "%local-database"
}

func (kvs *SymbolicLocalDatabase) WidestOfType() SymbolicValue {
	return &SymbolicLocalDatabase{}
}

///

func (kvs *LocalDatabase) ToSymbolicValue(wide bool, encountered map[uintptr]SymbolicValue) (SymbolicValue, error) {
	return &SymbolicLocalDatabase{}, nil
}
