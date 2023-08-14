package utils

import (
	"fmt"
	"math"

	"golang.org/x/exp/constraints"
)

func Min[T constraints.Ordered](a T, b T) T {
	return min(a, b)
}

func Max[T constraints.Ordered](a, b T) T {
	return max(a, b)
}

func Abs[T constraints.Integer](a T) T {
	if a < 0 {
		if a == -a {
			panic(fmt.Errorf("%d has no absolute value", a))
		}
		return -a
	}
	return a
}

func CountDigits(n int64) int {
	count := 0
	if n == math.MinInt64 {
		n += 1
	}
	if n < 0 {
		n = -n
	}

	for n >= 10 {
		n /= 10
		count++
	}
	count++

	return count
}
