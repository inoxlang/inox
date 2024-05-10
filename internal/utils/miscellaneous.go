package utils

import (
	"errors"
	"fmt"
	"reflect"
	"runtime/debug"
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

func PanicIfErrAmong(errs ...error) {
	err := errors.Join(errs...)
	if err != nil {
		panic(err)
	}
}

func Catch(fn func()) (result error) {
	defer func() {
		e := recover()
		if e != nil {
			err := ConvertPanicValueToError(result)
			result = fmt.Errorf("%w: %s", err, string(debug.Stack()))
		}
	}()
	fn()
	return nil
}

func Ret0[A, B any](a A, b B) A {
	return a
}

func Ret1[A, B any](a A, b B) B {
	return b
}

func Ret0OutOf3[A, B, C any](a A, b B, c C) A {
	return a
}

func SamePointer(a, b interface{}) bool {
	return reflect.ValueOf(a).Pointer() == reflect.ValueOf(b).Pointer()
}

func SameIdentityStrings(a, b string) bool {
	header1 := (*reflect.StringHeader)(unsafe.Pointer(&a))
	header2 := (*reflect.StringHeader)(unsafe.Pointer(&b))
	return *header1 == *header2
}

func GetByteSize[T any]() uintptr {
	var v T
	return unsafe.Sizeof(v)
}

// If cond is true a is returned, else b is returned.
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

func Implements[T any](v any) bool {
	_, ok := v.(T)
	return ok
}

func New[T any](v T) *T {
	ptr := new(T)
	*ptr = v
	return ptr
}
