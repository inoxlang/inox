package utils

import (
	"sync/atomic"
	"time"
)

// InefficientlyWaitUntilTrue checks every ~millisecond if *b is true,
// it returns true if *b is true or returns false on timeout.
func InefficientlyWaitUntilTrue(b *atomic.Bool, timeout time.Duration) bool {
	start := time.Now()
	for !b.Load() && time.Since(start) <= timeout {
		//wait at least one millisecond
		time.Sleep(time.Millisecond)
	}

	return b.Load()
}
