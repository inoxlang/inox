package core

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"unsafe"

	"github.com/inoxlang/inox/internal/core/mem"
	"github.com/inoxlang/inox/internal/core/patternnames"
	"github.com/inoxlang/inox/internal/core/symbolic"
)

var (
	BOOL_COMPTIME_TYPE = &BuiltinType{
		name:     patternnames.BOOL,
		symbolic: symbolic.BUILTIN_COMPTIME_TYPES[patternnames.BOOL],
		goType:   BOOL_TYPE,
	}
	INT_COMPTIME_TYPE = &BuiltinType{
		name:     patternnames.INT,
		symbolic: symbolic.BUILTIN_COMPTIME_TYPES[patternnames.INT],
		goType:   INT_TYPE,
	}
	FLOAT_COMPTIME_TYPE = &BuiltinType{
		name:     patternnames.FLOAT,
		symbolic: symbolic.BUILTIN_COMPTIME_TYPES[patternnames.FLOAT],
		goType:   FLOAT64_TYPE,
	}
	STRING_COMPTIME_TYPE = &BuiltinType{
		name:     patternnames.STRING,
		symbolic: symbolic.BUILTIN_COMPTIME_TYPES[patternnames.STRING],
		goType:   STRING_TYPE,
	}

	_ = []CompileTimeType{}
)

type CompileTimeType interface {
	Symbolic() symbolic.CompileTimeType
	GoType() reflect.Type
}

type ModuleComptimeTypes struct {
	structTypes        map[string]*StructType
	symbolicToConcrete map[symbolic.CompileTimeType]CompileTimeType
	symbolic           *symbolic.ModuleCompileTimeTypes
}

func NewModuleComptimeTypes(symb *symbolic.ModuleCompileTimeTypes) *ModuleComptimeTypes {
	types := &ModuleComptimeTypes{
		structTypes:        make(map[string]*StructType, 0),
		symbolicToConcrete: make(map[symbolic.CompileTimeType]CompileTimeType, 0),
	}
	if symb == nil {
		symb = symbolic.NewModuleCompileTimeTypes()
	}
	types.symbolic = symb

	return types
}

func (types *ModuleComptimeTypes) getConcreteType(t symbolic.CompileTimeType) (result CompileTimeType) {
	return types._getConcreteType(t, -1)
}

func (types *ModuleComptimeTypes) getConcretePointerTypeByName(name string) (*PointerType, bool) {
	pointerType, ok := types.symbolic.GetPointerType(name)
	if !ok {
		return nil, false
	}
	return types.getConcreteType(pointerType).(*PointerType), true
}

func (types *ModuleComptimeTypes) _getConcreteType(t symbolic.CompileTimeType, depth int) (result CompileTimeType) {
	depth++
	if depth > 10 {
		panic(errors.New("type cycle"))
	}

	defer func() {
		e := recover()
		if e != nil {
			panic(e)
		}

		//If the type is a named struct type we check it is unique.

		structType, ok := t.(*symbolic.StructType)
		if !ok {
			return
		}
		name, ok := structType.Name()
		if !ok {
			return
		}
		storedType, ok := types.structTypes[name]
		if !ok {
			types.structTypes[name] = result.(*StructType)
			return
		}
		if storedType != result.(*StructType) {
			panic(errors.New("a struct type name should correspond to a single concrete struct type"))
		}
	}()

	concrete, ok := types.symbolicToConcrete[t]
	if ok {
		return concrete
	}

	switch t := t.(type) {
	case *symbolic.StructType:
		structType := &StructType{symbolic: t}
		concrete = structType
		types.symbolicToConcrete[t] = concrete

		var tempFields []reflect.StructField

		structPkgPath := "struct" + strconv.FormatUint(uint64(uintptr(unsafe.Pointer(structType))), 10)

		for i := 0; i < t.FieldCount(); i++ {
			symbolicField := t.Field(i)
			fieldType := types._getConcreteType(symbolicField.Type, depth)

			field := reflect.StructField{
				Name:    symbolicField.Name,
				Type:    fieldType.GoType(),
				PkgPath: structPkgPath,
				//Tag: ,
			}
			tempFields = append(tempFields, field)
		}

		if len(tempFields) == 0 {
			tempFields = append(tempFields, reflect.StructField{
				Name: "_",
				Type: BOOL_TYPE,
			})
		}

		//TODO: reorder the fields to minimize padding

		structType.goStructType = reflect.StructOf(tempFields)

		for i := 0; i < len(tempFields); i++ {
			structField := structType.goStructType.Field(i)

			info := fieldRetrievalInfo{
				name:   structField.Name,
				offset: int(structField.Offset),
			}

			switch structField.Type.Kind() {
			case reflect.Bool:
				info.typ = GetBoolField
			case reflect.Int64:
				info.typ = GetIntField
			case reflect.Float64:
				info.typ = GetFloatField
			case reflect.String:
				info.typ = GetStringField
			case reflect.Pointer:
				info.typ = GetStructPointerField
			default:
				panic(ErrUnreachable)
			}

			structType.fieldRetrievalInfo = append(structType.fieldRetrievalInfo, info)
		}

		return concrete
	case *symbolic.BoolType:
		concrete = BOOL_COMPTIME_TYPE
	case *symbolic.IntType:
		concrete = INT_COMPTIME_TYPE
	case *symbolic.FloatType:
		concrete = FLOAT_COMPTIME_TYPE
	case *symbolic.StringType:
		concrete = STRING_COMPTIME_TYPE
	case *symbolic.PointerType:
		valueType := types._getConcreteType(t.ValueType(), depth)
		concrete = &PointerType{
			symbolic:  t,
			valueType: valueType,
			goPtrType: reflect.PointerTo(valueType.GoType()),
		}
	}

	if concrete == nil {
		panic(errors.New("failed to get concrete type"))
	}

	types.symbolicToConcrete[t] = concrete
	return concrete
}

type StructType struct {
	symbolic           *symbolic.StructType
	goStructType       reflect.Type
	fieldRetrievalInfo []fieldRetrievalInfo
}

func (t *StructType) FieldCount() int {
	return t.symbolic.FieldCount()
}

func (t *StructType) FieldRetrievalInfo(name string) fieldRetrievalInfo {
	for _, fieldInfo := range t.fieldRetrievalInfo {
		if fieldInfo.name == name {
			return fieldInfo
		}
	}
	panic(fmt.Errorf("field '%s' does not exist in the struct", name))
}

func (t *StructType) Symbolic() symbolic.CompileTimeType {
	return t.symbolic
}

func (t *StructType) GoType() reflect.Type {
	return t.goStructType
}

type BuiltinType struct {
	name     string
	symbolic symbolic.CompileTimeType
	goType   reflect.Type
}

func (t *BuiltinType) Symbolic() symbolic.CompileTimeType {
	return t.symbolic
}

func (t *BuiltinType) GoType() reflect.Type {
	return t.goType
}

type PointerType struct {
	symbolic  *symbolic.PointerType
	valueType CompileTimeType
	goPtrType reflect.Type
}

func (t *PointerType) Symbolic() symbolic.CompileTimeType {
	return t.symbolic
}

func (t *PointerType) ValueType() CompileTimeType {
	return t.valueType
}

func (t *PointerType) ValueSize() uintptr {
	return t.valueType.GoType().Size()
}

func (t *PointerType) StructFieldRetrieval(name string) fieldRetrievalInfo {
	structType := t.ValueType().(*StructType)
	return structType.FieldRetrievalInfo(name)
}

func (t *PointerType) GoType() reflect.Type {
	return t.goPtrType
}

// New allocates the memory needed for the value and returns a pointer to it.
func (t *PointerType) New(heap *mem.ModuleHeap) mem.HeapAddress {
	size, alignment := t.GetValueAllocParams()
	return mem.Alloc[byte](heap, size, alignment)
}

func (t *PointerType) GetValueAllocParams() (size int, alignment int) {
	size = int(t.valueType.GoType().Size())
	if size == 0 {
		panic(fmt.Errorf("pointed value has a size of 0"))
	}
	alignment = t.goPtrType.Align()
	return
}
