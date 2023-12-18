package jsonrpc

import (
	"errors"
	"sync"
	"sync/atomic"
	"time"

	netaddr "github.com/inoxlang/inox/internal/netaddr"
	"github.com/inoxlang/inox/internal/ratelimit"
	"github.com/rs/zerolog"
)

var (
	DEFAULT_RATE_LIMITS = []int{5, 20, 80}
)

type rateLimiter struct {
	methodToLimiter map[string]*methodRateLimiter
	logger          zerolog.Logger
	lock            sync.Mutex
}

func newRateLimiter(logger zerolog.Logger) *rateLimiter {
	r := &rateLimiter{
		methodToLimiter: make(map[string]*methodRateLimiter, 0),
		logger:          logger,
	}
	return r
}

func (r *rateLimiter) limit(info MethodInfo, reqId string, addrPort netaddr.RemoteAddrWithPort) (rateLimited bool, methodRateLimited bool) {
	r.lock.Lock()
	defer r.lock.Unlock()

	now := time.Now()

	if info.Name == "" {
		panic(errors.New("unknown method"))
	}
	if info.Name == CANCEL_REQUEST_METHOD {
		return false, false
	}

	methodLimiter, ok := r.methodToLimiter[info.Name]

	//always allow first invocation
	if !ok {
		limiter := &methodRateLimiter{
			info: info,
		}
		limiter.lastInvocationTime.Store(time.Now())

		//the windows are not initialized to avoid unecessary allocation,
		//in the case the method is never called again.

		r.methodToLimiter[info.Name] = limiter
		return false, false
	}

	//create windows if necessary
	if len(methodLimiter.windows) == 0 {
		rateLimits := info.RateLimits
		if len(info.RateLimits) == 0 {
			rateLimits = DEFAULT_RATE_LIMITS
		}

		windowDuration := time.Second //scaled x10 each time

		for _, reqCount := range rateLimits {
			if reqCount == 0 {
				methodLimiter.windows = append(methodLimiter.windows, nil)
			} else {
				methodLimiter.windows = append(methodLimiter.windows, ratelimit.NewSlidingWindow(ratelimit.WindowParameters{
					Duration:     windowDuration,
					RequestCount: reqCount,
				}))
			}

			windowDuration *= 10
		}
	}

	limited := false

	for _, window := range methodLimiter.windows {
		if window == nil {
			continue
		}

		ok := window.AllowRequest(ratelimit.SlidingWindowRequestInfo{
			Id:                reqId,
			Method:            info.Name,
			CreationTime:      now,
			RemoteAddrAndPort: addrPort,
			RemoteIpAddr:      addrPort.RemoteIp(),
			SentBytes:         0,
		}, r.logger)

		if !ok {
			limited = true
			//we don't break because all windows should be informed
			//about the request.
		}
	}

	return limited, limited
}

type methodRateLimiter struct {
	info               MethodInfo
	lastInvocationTime atomic.Value
	windows            []ratelimit.IWindow
}

func (l *methodRateLimiter) LastInvocationTime() time.Time {
	return l.lastInvocationTime.Load().(time.Time)
}
