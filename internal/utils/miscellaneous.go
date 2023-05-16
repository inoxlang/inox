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

func PanicIfErr(err error) {
	if err != nil {
		panic(err)
	}
}

func Ret0[A, B any](a A, b B) A {
	return a
}

func Ret1[A, B any](a A, b B) B {
	return b
}

func SamePointer(a, b interface{}) bool {
	return reflect.ValueOf(a).Pointer() == reflect.ValueOf(b).Pointer()
}

func GetByteSize[T any]() uintptr {
	var v T
	return unsafe.Sizeof(v)
}

func If[T any](cond bool, a, b T) T {
	if cond {
		return a
	}
	return b
}

func Ret[V any](v V) func() V {
	return func() V {
		return v
	}
}
