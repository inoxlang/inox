package symbolic

import "github.com/inoxlang/inox/internal/core/patternnames"

var (
	BUILTIN_COMPTIME_TYPES = map[string]CompileTimeType{
		patternnames.BOOL:   &BoolType{},
		patternnames.INT:    &IntType{},
		patternnames.FLOAT:  &FloatType{},
		patternnames.STRING: &StringType{},
	}
)

func IsNameOfBuiltinComptimeType(name string) bool {
	_, ok := BUILTIN_COMPTIME_TYPES[name]
	return ok
}

type BoolType struct {
}

func (t *BoolType) Equal(v CompileTimeType, state RecTestCallState) bool {
	_, ok := v.(*BoolType)
	if !ok {
		return false
	}
	//add special bool types ?

	return true
}

func (t *BoolType) TestValue(v Value, state RecTestCallState) bool {
	return ImplementsOrIsMultivalueWithAllValuesImplementing[*Bool](v)
}

type IntType struct {
}

func (t *IntType) Equal(v CompileTimeType, state RecTestCallState) bool {
	_, ok := v.(*IntType)
	if !ok {
		return false
	}
	//add special bool types ?

	return true
}

func (t *IntType) TestValue(v Value, state RecTestCallState) bool {
	return ImplementsOrIsMultivalueWithAllValuesImplementing[*Int](v)
}

type FloatType struct {
}

func (t *FloatType) Equal(v CompileTimeType, state RecTestCallState) bool {
	_, ok := v.(*FloatType)
	if !ok {
		return false
	}
	//add special bool types ?

	return true
}

func (t *FloatType) TestValue(v Value, state RecTestCallState) bool {
	return ImplementsOrIsMultivalueWithAllValuesImplementing[*Float](v)
}

type StringType struct {
}

func (t *StringType) Equal(v CompileTimeType, state RecTestCallState) bool {
	_, ok := v.(*StringType)
	if !ok {
		return false
	}
	//add special bool types ?

	return true
}

func (t *StringType) TestValue(v Value, state RecTestCallState) bool {
	return ImplementsOrIsMultivalueWithAllValuesImplementing[*String](v)
}
