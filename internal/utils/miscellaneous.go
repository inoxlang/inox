package utils

import (
	"reflect"
	"unsafe"
)

func Must[T any](obj T, err error) T {
	if err != nil {
		panic(err)
	}
	return obj
}

func Ret0[A, B any](a A, b B) A {
	return a
}

func SamePointer(a, b interface{}) bool {
	return reflect.ValueOf(a).Pointer() == reflect.ValueOf(b).Pointer()
}

func CopyMap[K comparable, V any](m map[K]V) map[K]V {
	mapCopy := make(map[K]V, len(m))

	for k, v := range m {
		mapCopy[k] = v
	}

	return mapCopy
}

func GetByteSize[T any]() uintptr {
	var v T
	return unsafe.Sizeof(v)
}
