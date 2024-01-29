package testconfig

import "testing"

var (
	PARELLIZE_SAME_PKG_TESTS = false
)

func AllowParallelization(t *testing.T) {
	if PARELLIZE_SAME_PKG_TESTS {
		t.Parallel()
	}
}
