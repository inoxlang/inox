package intconv

import (
	"fmt"
	"math"
	"unsafe"

	"github.com/inoxlang/inox/internal/utils"
)

var (
	intIsInt64 = unsafe.Sizeof(int(0)) == 8
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
