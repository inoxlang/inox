package utils

import "golang.org/x/exp/constraints"

func Min[T constraints.Ordered](a, b T) T {
	if a < b {
		return a
	}
	return b
}

func Max[T constraints.Ordered](a, b T) T {
	if a > b {
		return a
	}
	return b
}

func Abs[T constraints.Integer](a T) T {
	if a < 0 {
		return -a
	}
	return a
}
