package intconv

import (
	"fmt"
	"math"
	"unsafe"

	utils "github.com/inoxlang/inox/internal/utils/common"
)

var (
	intIsInt64 = unsafe.Sizeof(int(0)) == 8
	intIsInt32 = !intIsInt64
)

func I64ToI(i int64) (int, error) {
	if intIsInt64 {
		return int(i), nil
	}
	if i > math.MaxInt || i < math.MinInt {
		return 0, fmt.Errorf("%d is out the int (int32) range", i)
	}
	return int(i), nil
}

func MustI64ToI(i int64) int {
	return utils.Must(I64ToI(i))
}

func IToI32(i int) (int32, error) {
	if intIsInt32 {
		return int32(i), nil
	}
	if i > math.MaxInt32 || i < math.MinInt32 {
		return 0, fmt.Errorf("%d is out the int (int32) range", i)
	}
	return int32(i), nil
}

func MustIToI32(i int) int32 {
	return utils.Must(IToI32(i))
}
