package core

import (
	"testing"
	"unsafe"

	"github.com/inoxlang/inox/internal/core/mem"
	"github.com/inoxlang/inox/internal/core/symbolic"
	utils "github.com/inoxlang/inox/internal/utils/common"
	"github.com/stretchr/testify/assert"
)

func TestPointerTypeNew(t *testing.T) {
	symbolicTypes := symbolic.NewModuleCompileTimeTypes()

	structType1 := symbolic.NewStructType("FourBoolFieldsStruct", []symbolic.StructField{
		{Name: "a", Type: symbolic.BUILTIN_COMPTIME_TYPES["bool"]},
		{Name: "b", Type: symbolic.BUILTIN_COMPTIME_TYPES["bool"]},
		{Name: "c", Type: symbolic.BUILTIN_COMPTIME_TYPES["bool"]},
		{Name: "d", Type: symbolic.BUILTIN_COMPTIME_TYPES["bool"]},
	}, nil)
	symbolicTypes.DefineType("FourBoolFieldsStruct", structType1)

	structType2 := symbolic.NewStructType("BoolIntFieldsStruct", []symbolic.StructField{
		{Name: "a", Type: symbolic.BUILTIN_COMPTIME_TYPES["bool"]},
		{Name: "b", Type: symbolic.BUILTIN_COMPTIME_TYPES["int"]},
	}, nil)
	symbolicTypes.DefineType("BoolIntFieldsStruct", structType2)

	types := NewModuleComptimeTypes(symbolicTypes)
	heap := mem.NewArenaHeap(100)
	heap.Alloc(1, 1)

	{
		intPtrType := utils.MustGet(types.getConcretePointerTypeByName("int"))
		addr := intPtrType.New(heap)
		ptr := uintptr(unsafe.Pointer(addr))
		assert.Zero(t, ptr%8)
	}

	heap.Alloc(1, 1)

	{
		floatPtrType := utils.MustGet(types.getConcretePointerTypeByName("float"))
		addr := floatPtrType.New(heap)
		ptr := uintptr(unsafe.Pointer(addr))
		assert.Zero(t, ptr%8)
	}

	heap.Alloc(1, 1)

	{
		structType := utils.MustGet(types.getConcretePointerTypeByName("FourBoolFieldsStruct"))
		addr := structType.New(heap)
		ptr := uintptr(unsafe.Pointer(addr))
		assert.Zero(t, ptr%4)
	}

	heap.Alloc(1, 1)

	{
		structType := utils.MustGet(types.getConcretePointerTypeByName("BoolIntFieldsStruct"))
		addr := structType.New(heap)
		ptr := uintptr(unsafe.Pointer(addr))
		assert.Zero(t, ptr%8)
	}
}
