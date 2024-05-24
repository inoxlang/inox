package testconfig

import "testing"

var (
	PARALLELIZE_SAME_PKG_TESTS = false
)

func AllowParallelization(t *testing.T) {
	if PARALLELIZE_SAME_PKG_TESTS {
		t.Parallel()
	}
}
