package utils

import (
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

const (
	defaultPreSleepDuration = 20 * time.Millisecond
)

type AssertNoMemoryLeakOptions struct {
	// duration of sleep before collection of memory stats,
	// defaults to defaultPreSleepDuration.
	PreSleepDurationMillis uint8

	// (optional) number of goroutines at the beginning,
	// if set AssertNoMemoryLeak checks that the number of goroutines has not increased.
	GoroutineCount int

	MaxGoroutineCountDelta uint8
}

// AssertNoMemoryLeak checks that at most maxAllocDelta bytes have been allocated since the passed
// memory stats have been collected. This function should be called at the end of a long test suite or test case.
func AssertNoMemoryLeak(t *testing.T, startStats *runtime.MemStats, maxAllocDelta uint64, opts ...AssertNoMemoryLeakOptions) {
	runtime.GC()

	slept := false
	if len(opts) > 0 {
		if opts[0].PreSleepDurationMillis > 0 {
			time.Sleep(time.Duration(opts[0].PreSleepDurationMillis) * time.Millisecond)
			slept = true
		}
	}

	if !slept {
		time.Sleep(defaultPreSleepDuration)
	}

	runtime.GC()

	memStats := new(runtime.MemStats)
	runtime.ReadMemStats(memStats)

	if len(opts) > 0 && opts[0].GoroutineCount > 0 {
		delta := runtime.NumGoroutine() - opts[0].GoroutineCount
		if delta > int(opts[0].MaxGoroutineCountDelta) {
			assert.FailNowf(t, "goroutine leaks", "%d goroutines leaking", delta)
		}
	}

	if startStats.Alloc > memStats.Alloc {
		return
	}

	delta := memStats.Alloc - startStats.Alloc
	if delta > maxAllocDelta {
		failureMsg := "memory leak"

		if delta > 1_000_000 {
			assert.FailNowf(t, failureMsg, "%d MB", delta/uint64(1_000_000))
		} else if delta > 1_000 {
			assert.FailNowf(t, failureMsg, "%d kB", delta/uint64(1_000))
		} else {
			assert.FailNowf(t, failureMsg, "%d B", delta)
		}
	}
}
