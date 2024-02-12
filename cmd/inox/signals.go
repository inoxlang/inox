package main

import (
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/inoxlang/inox/internal/core"
)

const MAX_UNGRACEFUL_TEARDOWN_DURATION = 100 * time.Millisecond

// CancelOnSigintSigterm creates a goroutine that catches SIGINT and SIGTERM signals. On reception of a signal
// the goroutine gracefully cancels $ctx. If the graceful teardown successfully completes, os.Exit(0) is called.
// If the $ctx is still not done after $teardownTimeout, $ctx.CancelUngracefully is called. os.Exit(128+signal)
// is called at most MAX_UNGRACEFUL_TEARDOWN_DURATION after.
func CancelOnSigintSigterm(ctx *core.Context, teardownTimeout time.Duration) {
	// Cancel the context on SIGINT or SIGTERM.
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM /*All listed signals should be in the switch statement further below.*/)

	go func() {
		for signal := range ch {
			var s int
			switch signal {
			case syscall.SIGINT:
				s = int(syscall.SIGINT)
			case syscall.SIGTERM:
				s = int(syscall.SIGTERM)
			}

			go func() {
				<-time.After(teardownTimeout)

				go func() {
					<-time.After(MAX_UNGRACEFUL_TEARDOWN_DURATION)
					os.Exit(128 + s) //https://tldp.org/LDP/abs/html/exitcodes.html
				}()
				ctx.CancelUngracefully()
			}()

			ctx.CancelGracefully()
			os.Exit(0) //success
		}
	}()

}
