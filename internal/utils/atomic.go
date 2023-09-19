package utils

import (
	"sync/atomic"
	"time"
)

func InefficientlyWaitUntilTrue(b *atomic.Bool, timeout time.Duration) bool {
	start := time.Now()
	for !b.Load() && time.Since(start) <= timeout {
		time.Sleep(time.Millisecond)
	}

	return b.Load()
}
