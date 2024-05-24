//go:build parallelsamepkgtests

package testconfig

func init() {
	PARALLELIZE_SAME_PKG_TESTS = true
}
