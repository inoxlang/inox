package utils

import (
	"errors"
	"reflect"
	"unsafe"
)

func Must[T any](a T, err error) T {
	if err != nil {
		panic(err)
	}
	return a
}

func Must2[T any, U any](a T, b U, err error) (T, U) {
	if err != nil {
		panic(err)
	}
	return a, b
}

func MustGet[T any](a T, found bool) T {
	if !found {
		panic(errors.New("not found"))
	}
	return a
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
