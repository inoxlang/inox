package symbolic

var _ = []CompileTimeType{
	(*StructType)(nil), (*PointerType)(nil), (*IntType)(nil), (*FloatType)(nil),
	(*BoolType)(nil), (*StringType)(nil),
}

type CompileTimeType interface {
	Equal(v CompileTimeType, state RecTestCallState) bool
	TestValue(v Value, state RecTestCallState) bool
	SymbolicValue() Value
}

type ModuleCompileTimeTypes struct {
	all          map[string]CompileTimeType
	pointerTypes map[ /*value type*/ string]*PointerType
}

func newModuleCompileTimeTypes() *ModuleCompileTimeTypes {
	types := &ModuleCompileTimeTypes{
		all:          make(map[string]CompileTimeType, len(BUILTIN_COMPTIME_TYPES)),
		pointerTypes: make(map[string]*PointerType, 0),
	}

	for name, comptimeType := range BUILTIN_COMPTIME_TYPES {
		types.all[name] = comptimeType
	}

	return types
}

func (t *ModuleCompileTimeTypes) IsTypeDefined(name string) bool {
	_, ok := t.all[name]
	return ok
}

func (t *ModuleCompileTimeTypes) DefineType(name string, typ CompileTimeType) {
	_, ok := t.all[name]
	if ok {
		panic(ErrComptimeTypeAlreadyDefined)
	}
	t.all[name] = typ
}

func (t *ModuleCompileTimeTypes) GetType(typename string) (CompileTimeType, bool) {
	typ, ok := t.all[typename]
	return typ, ok
}

func (t *ModuleCompileTimeTypes) GetPointerType(valueTypename string) (*PointerType, bool) {
	typ, ok := t.all[valueTypename]
	if !ok {
		return nil, false
	}

	ptrType, ok := t.pointerTypes[valueTypename]
	if !ok {
		ptrType = newPointerType(typ)
		t.pointerTypes[valueTypename] = ptrType
	}

	return ptrType, true
}
