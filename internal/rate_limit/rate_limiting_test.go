package ratelimit

import (
	"os"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

func TestSlidingWindow(t *testing.T) {

	logger := zerolog.New(os.Stdout)

	windowParams := WindowParameters{
		Duration:     3 * time.Second,
		RequestCount: 3,
	}

	t.Run("add request to empty sliding window", func(t *testing.T) {
		window := NewSlidingWindow(windowParams)

		assert.True(t, window.AllowRequest(SlidingWindowRequestInfo{
			CreationTime: time.Unix(0, 1),
			ULID:         ulid.Make(),
		}, logger))
	})

	t.Run("add request to full sliding window : oldest request was received less than <window duration> ago", func(t *testing.T) {
		window := NewSlidingWindow(windowParams)

		window.AllowRequest(SlidingWindowRequestInfo{
			CreationTime: time.Unix(0, 1),
			ULID:         ulid.Make(),
		}, logger)

		window.AllowRequest(SlidingWindowRequestInfo{
			CreationTime: time.Unix(1, 0),
			ULID:         ulid.Make(),
		}, logger)

		window.AllowRequest(SlidingWindowRequestInfo{
			CreationTime: time.Unix(2, 0),
			ULID:         ulid.Make(),
		}, logger)

		assert.False(t, window.AllowRequest(SlidingWindowRequestInfo{
			CreationTime: time.Unix(2, 0),
			ULID:         ulid.Make(),
		}, logger))
	})

	t.Run("add request to full sliding window : oldest request was received less than <window duration> ago"+
		"additional requests are allowed if IP is not sending many requests", func(t *testing.T) {
		params := windowParams
		window := NewSlidingWindow(params)
		window.ipLevelWindow = NewSlidingWindow(WindowParameters{
			Duration:     params.Duration,
			RequestCount: 1,
		})

		window.AllowRequest(SlidingWindowRequestInfo{
			CreationTime: time.Unix(0, 1),
			ULID:         ulid.Make(),
		}, logger)

		window.AllowRequest(SlidingWindowRequestInfo{
			CreationTime: time.Unix(1, 0),
			ULID:         ulid.Make(),
		}, logger)

		window.AllowRequest(SlidingWindowRequestInfo{
			CreationTime: time.Unix(2, 0),
			ULID:         ulid.Make(),
		}, logger)

		assert.True(t, window.AllowRequest(SlidingWindowRequestInfo{
			CreationTime: time.Unix(2, 0),
			ULID:         ulid.Make(),
		}, logger))
	})

	t.Run("add request to full sliding window : oldest request was received more than <window duration> ago", func(t *testing.T) {
		window := NewSlidingWindow(windowParams)

		window.AllowRequest(SlidingWindowRequestInfo{
			CreationTime: time.Unix(0, 1),
			ULID:         ulid.Make(),
		}, logger)

		window.AllowRequest(SlidingWindowRequestInfo{
			CreationTime: time.Unix(2, 0),
			ULID:         ulid.Make(),
		}, logger)

		window.AllowRequest(SlidingWindowRequestInfo{
			CreationTime: time.Unix(3, 0),
			ULID:         ulid.Make(),
		}, logger)

		assert.True(t, window.AllowRequest(SlidingWindowRequestInfo{
			CreationTime: time.Unix(4, 0),
			ULID:         ulid.Make(),
		}, logger))
	})
}

func TestSharedSlidingWindow(t *testing.T) {

	logger := zerolog.New(os.Stdout)

	windowParams := WindowParameters{
		Duration:     3 * time.Second,
		RequestCount: 3,
	}

	t.Run("add request to empty sliding window", func(t *testing.T) {
		window := NewSharedRateLimitingWindow(windowParams)

		assert.True(t, window.AllowRequest(SlidingWindowRequestInfo{
			CreationTime:      time.Unix(0, 1),
			ULID:              ulid.Make(),
			RemoteAddrAndPort: "37.00.00.01:3000",
		}, logger))
	})

	t.Run("N requests (N > <req count> / 2) from same ip:port, last request should be blocked", func(t *testing.T) {
		window := NewSharedRateLimitingWindow(windowParams)

		window.AllowRequest(SlidingWindowRequestInfo{
			CreationTime:      time.Unix(1000, 1),
			ULID:              ulid.Make(),
			RemoteAddrAndPort: "37.00.00.01:3000",
		}, logger)

		assert.False(t, window.AllowRequest(SlidingWindowRequestInfo{
			CreationTime:      time.Unix(1001, 0),
			ULID:              ulid.Make(),
			RemoteAddrAndPort: "37.00.00.01:3000",
		}, logger))

	})

	t.Run("sliding window is full of requests less than <window duration> old from ip:port A : new request with ip:port B should be blocked", func(t *testing.T) {
		window := NewSharedRateLimitingWindow(windowParams)

		window.AllowRequest(SlidingWindowRequestInfo{
			CreationTime:      time.Unix(1000, 1),
			ULID:              ulid.Make(),
			RemoteAddrAndPort: "37.00.00.01:3000",
		}, logger)

		window.AllowRequest(SlidingWindowRequestInfo{
			CreationTime:      time.Unix(1001, 0),
			ULID:              ulid.Make(),
			RemoteAddrAndPort: "37.00.00.01:3000",
		}, logger)

		window.AllowRequest(SlidingWindowRequestInfo{
			CreationTime:      time.Unix(1002, 0),
			ULID:              ulid.Make(),
			RemoteAddrAndPort: "37.00.00.01:3000",
		}, logger)

		assert.False(t, window.AllowRequest(SlidingWindowRequestInfo{
			CreationTime:      time.Unix(1002, 10),
			ULID:              ulid.Make(),
			RemoteAddrAndPort: "37.00.00.01:3001",
		}, logger))
	})

	t.Run("sliding window is full of requests less than <window duration> old from ip:port A (except one that is older) : new request with ip:port B should not be blocked", func(t *testing.T) {
		window := NewSharedRateLimitingWindow(windowParams)

		window.AllowRequest(SlidingWindowRequestInfo{
			CreationTime:      time.Unix(1000, 0),
			ULID:              ulid.Make(),
			RemoteAddrAndPort: "37.00.00.01:3000",
		}, logger)

		window.AllowRequest(SlidingWindowRequestInfo{
			CreationTime:      time.Unix(1002, 0),
			ULID:              ulid.Make(),
			RemoteAddrAndPort: "37.00.00.01:3000",
		}, logger)

		window.AllowRequest(SlidingWindowRequestInfo{
			CreationTime:      time.Unix(1003, 0),
			ULID:              ulid.Make(),
			RemoteAddrAndPort: "37.00.00.01:3000",
		}, logger)

		//request from other IP
		assert.True(t, window.AllowRequest(SlidingWindowRequestInfo{
			CreationTime:      time.Unix(1004, 0),
			ULID:              ulid.Make(),
			RemoteAddrAndPort: "37.00.00.02:3001",
		}, logger))
	})

}
