package jsonrpc

import (
	"strconv"
	"testing"
	"time"

	netaddr "github.com/inoxlang/inox/internal/netaddr"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/oklog/ulid/v2"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

func TestRateLimiter(t *testing.T) {
	t.Run("the total number of messages per second should be limited", func(t *testing.T) {
		limiter := newRateLimiter(zerolog.Nop(), rateLimiterConfig{})
		addrPort := utils.Must(netaddr.RemoteAddrWithPortFrom("37.00.00.01:3000"))

		rateLimit0 := DEFAULT_MESSAGE_RATE_LIMITS[0] //rate limit for a 1s window.
		rateLimit1 := DEFAULT_MESSAGE_RATE_LIMITS[1]

		//Check rate limiting for the 1s window.

		for i := 0; i < rateLimit0; i++ {
			rateLimited, methodRateLimited := limiter.limit(MethodInfo{Name: strconv.Itoa(i)}, ulid.Make().String(), addrPort)
			assert.False(t, rateLimited)
			assert.False(t, methodRateLimited)
		}

		rateLimited, methodRateLimited := limiter.limit(MethodInfo{Name: "0"}, ulid.Make().String(), addrPort)
		assert.True(t, rateLimited)
		assert.False(t, methodRateLimited)

		time.Sleep(time.Second)

		//Check rate limiting for the 10s window.

		i := 0
		for ; i < rateLimit1; i++ {
			//Wait a bit in order to not be rate limited by the 1s window and burst-limited by the 10s window.
			time.Sleep(3 * time.Second / time.Duration(rateLimit0))

			rateLimited, methodRateLimited := limiter.limit(MethodInfo{Name: strconv.Itoa(i)}, ulid.Make().String(), addrPort)

			shouldBeRateLimited := i > rateLimit1
			if shouldBeRateLimited {
				if !assert.True(t, rateLimited, strconv.Itoa(i)) {
					return
				}
				if !assert.True(t, methodRateLimited, strconv.Itoa(i)) {
					return
				}
			} else {
				if !assert.False(t, rateLimited, strconv.Itoa(i)) {
					return
				}
				if !assert.False(t, methodRateLimited, strconv.Itoa(i)) {
					return
				}
			}
		}
	})
}
