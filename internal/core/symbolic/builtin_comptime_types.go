package symbolic

import (
	"github.com/inoxlang/inox/internal/core/patternnames"
	pprint "github.com/inoxlang/inox/internal/prettyprint"
)

var (
	BUILTIN_COMPTIME_TYPES = map[string]CompileTimeType{
		patternnames.BOOL:   &BoolType{baseValue: ANY_BOOL},
		patternnames.INT:    &IntType{baseValue: ANY_INT},
		patternnames.FLOAT:  &FloatType{baseValue: ANY_FLOAT},
		patternnames.STRING: &StringType{baseValue: ANY_STRING},
	}
)

func IsNameOfBuiltinComptimeType(name string) bool {
	_, ok := BUILTIN_COMPTIME_TYPES[name]
	return ok
}

type BoolType struct {
	baseValue Value
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

func (t *BoolType) SymbolicValue() Value {
	return t.baseValue
}

func (t *BoolType) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	//add base value ?
	w.WriteString("bool-type")
}

type IntType struct {
	baseValue Value
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

func (t *IntType) SymbolicValue() Value {
	return t.baseValue
}

func (t *IntType) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	//add base value ?
	w.WriteString("int-type")
}

type FloatType struct {
	baseValue Value
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

func (t *FloatType) SymbolicValue() Value {
	return t.baseValue
}

func (t *FloatType) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	//add base value ?
	w.WriteString("float-type")
}

type StringType struct {
	baseValue Value
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

func (t *StringType) SymbolicValue() Value {
	return t.baseValue
}

func (t *StringType) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	//add base value ?
	w.WriteString("string-type")
}
