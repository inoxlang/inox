package utils

import (
	"unsafe"

	"golang.org/x/exp/constraints"
)

func CopySlice[T any](s []T) []T {
	sliceCopy := make([]T, len(s))
	copy(sliceCopy, s)

	return sliceCopy
}

func ReversedSlice[T any](s []T) []T {
	reversed := make([]T, len(s))
	copy(reversed, s)

	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		reversed[i], reversed[j] = reversed[j], reversed[i]
	}

	return reversed
}

func Reverse[T any](slice []T) {
	length := len(slice)

	for i, j := 0, length-1; i < j; i, j = i+1, j-1 {
		slice[i], slice[j] = slice[j], slice[i]
	}
}

func SliceContains[T constraints.Ordered](slice []T, v T) bool {
	for _, e := range slice {
		if e == v {
			return true
		}
	}

	return false
}

func MapSlice[T any, U any](s []T, mapper func(e T) U) []U {
	result := make([]U, len(s))

	for i, e := range s {
		result[i] = mapper(e)
	}

	return result
}

func MapSliceIndexed[T any, U any](s []T, mapper func(e T, i int) U) []U {
	result := make([]U, len(s))

	for i, e := range s {
		result[i] = mapper(e, i)
	}

	return result
}

func FilterSlice[T any](s []T, filter func(e T) bool) []T {
	result := make([]T, 0)

	for _, e := range s {
		if filter(e) {
			result = append(result, e)
		}
	}

	return result
}

func FilterMapSlice[T any, U any](s []T, mapper func(e T) (U, bool)) []U {
	var result []U

	for _, e := range s {
		res, keep := mapper(e)
		if keep {
			result = append(result, res)
		}
	}

	return result
}

func Some[T any](s []T, predicate func(e T) bool) bool {
	for _, e := range s {
		if predicate(e) {
			return true
		}
	}

	return false
}

func EmptySliceIfNil[T any](slice []T) []T {
	if slice == nil {
		return make([]T, 0)
	}
	return slice
}

func BytesAsString(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	return unsafe.String(unsafe.SliceData(b), len(b))
}

func StringAsBytes[T ~string](s T) []byte {
	return unsafe.Slice(unsafe.StringData(string(s)), len(s))
}
