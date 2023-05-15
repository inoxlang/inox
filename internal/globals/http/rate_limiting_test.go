package internal

import (
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/assert"
)

func TestSlidingWindow(t *testing.T) {

	windowParams := rateLimitingWindowParameters{
		duration:     3 * time.Second,
		requestCount: 3,
	}

	t.Run("add request to empty sliding window", func(t *testing.T) {
		window := newRateLimitingSlidingWindow(windowParams)

		assert.True(t, window.allowRequest(slidingWindowRequestInfo{
			creationTime: time.Unix(0, 1),
			ulid:         ulid.Make(),
		}))
	})

	t.Run("add request to full sliding window : oldest request was received less than <window duration> ago", func(t *testing.T) {
		window := newRateLimitingSlidingWindow(windowParams)

		window.allowRequest(slidingWindowRequestInfo{
			creationTime: time.Unix(0, 1),
			ulid:         ulid.Make(),
		})

		window.allowRequest(slidingWindowRequestInfo{
			creationTime: time.Unix(1, 0),
			ulid:         ulid.Make(),
		})

		window.allowRequest(slidingWindowRequestInfo{
			creationTime: time.Unix(2, 0),
			ulid:         ulid.Make(),
		})

		assert.False(t, window.allowRequest(slidingWindowRequestInfo{
			creationTime: time.Unix(2, 0),
			ulid:         ulid.Make(),
		}))
	})

	t.Run("add request to full sliding window : oldest request was received less than <window duration> ago, bursts are allowed", func(t *testing.T) {
		params := windowParams
		window := newRateLimitingSlidingWindow(params)
		window.burstWindow = newRateLimitingSlidingWindow(rateLimitingWindowParameters{
			duration:     params.duration,
			requestCount: 1,
		})

		window.allowRequest(slidingWindowRequestInfo{
			creationTime: time.Unix(0, 1),
			ulid:         ulid.Make(),
		})

		window.allowRequest(slidingWindowRequestInfo{
			creationTime: time.Unix(1, 0),
			ulid:         ulid.Make(),
		})

		window.allowRequest(slidingWindowRequestInfo{
			creationTime: time.Unix(2, 0),
			ulid:         ulid.Make(),
		})

		assert.True(t, window.allowRequest(slidingWindowRequestInfo{
			creationTime: time.Unix(2, 0),
			ulid:         ulid.Make(),
		}))
	})

	t.Run("add request to full sliding window : oldest request was received more than <window duration> ago", func(t *testing.T) {
		window := newRateLimitingSlidingWindow(windowParams)

		window.allowRequest(slidingWindowRequestInfo{
			creationTime: time.Unix(0, 1),
			ulid:         ulid.Make(),
		})

		window.allowRequest(slidingWindowRequestInfo{
			creationTime: time.Unix(2, 0),
			ulid:         ulid.Make(),
		})

		window.allowRequest(slidingWindowRequestInfo{
			creationTime: time.Unix(3, 0),
			ulid:         ulid.Make(),
		})

		assert.True(t, window.allowRequest(slidingWindowRequestInfo{
			creationTime: time.Unix(4, 0),
			ulid:         ulid.Make(),
		}))
	})
}

func TestSharedSlidingWindow(t *testing.T) {

	windowParams := rateLimitingWindowParameters{
		duration:     3 * time.Second,
		requestCount: 3,
	}

	t.Run("add request to empty sliding window", func(t *testing.T) {
		window := newSharedRateLimitingWindow(windowParams)

		assert.True(t, window.allowRequest(slidingWindowRequestInfo{
			creationTime:      time.Unix(0, 1),
			ulid:              ulid.Make(),
			remoteAddrAndPort: "37.00.00.01:3000",
		}))
	})

	t.Run("N requests (N > <req count> / 2) from same ip:port, last request should be blocked", func(t *testing.T) {
		window := newSharedRateLimitingWindow(windowParams)

		window.allowRequest(slidingWindowRequestInfo{
			creationTime:      time.Unix(1000, 1),
			ulid:              ulid.Make(),
			remoteAddrAndPort: "37.00.00.01:3000",
		})

		assert.False(t, window.allowRequest(slidingWindowRequestInfo{
			creationTime:      time.Unix(1001, 0),
			ulid:              ulid.Make(),
			remoteAddrAndPort: "37.00.00.01:3000",
		}))

	})

	t.Run("sliding window is full of requests less than <window duration> old from ip:port A : new request with ip:port B should be blocked", func(t *testing.T) {
		window := newSharedRateLimitingWindow(windowParams)

		window.allowRequest(slidingWindowRequestInfo{
			creationTime:      time.Unix(1000, 1),
			ulid:              ulid.Make(),
			remoteAddrAndPort: "37.00.00.01:3000",
		})

		window.allowRequest(slidingWindowRequestInfo{
			creationTime:      time.Unix(1001, 0),
			ulid:              ulid.Make(),
			remoteAddrAndPort: "37.00.00.01:3000",
		})

		window.allowRequest(slidingWindowRequestInfo{
			creationTime:      time.Unix(1002, 0),
			ulid:              ulid.Make(),
			remoteAddrAndPort: "37.00.00.01:3000",
		})

		assert.False(t, window.allowRequest(slidingWindowRequestInfo{
			creationTime:      time.Unix(1002, 10),
			ulid:              ulid.Make(),
			remoteAddrAndPort: "37.00.00.01:3001",
		}))
	})

	t.Run("sliding window is full of requests less than <window duration> old from ip:port A (except one that is older) : new request with ip:port B should not be blocked", func(t *testing.T) {
		window := newSharedRateLimitingWindow(windowParams)

		window.allowRequest(slidingWindowRequestInfo{
			creationTime:      time.Unix(1000, 0),
			ulid:              ulid.Make(),
			remoteAddrAndPort: "37.00.00.01:3000",
		})

		window.allowRequest(slidingWindowRequestInfo{
			creationTime:      time.Unix(1002, 0),
			ulid:              ulid.Make(),
			remoteAddrAndPort: "37.00.00.01:3000",
		})

		window.allowRequest(slidingWindowRequestInfo{
			creationTime:      time.Unix(1003, 0),
			ulid:              ulid.Make(),
			remoteAddrAndPort: "37.00.00.01:3000",
		})

		assert.True(t, window.allowRequest(slidingWindowRequestInfo{
			creationTime:      time.Unix(1004, 0),
			ulid:              ulid.Make(),
			remoteAddrAndPort: "37.00.00.02:3001",
		}))
	})

}
