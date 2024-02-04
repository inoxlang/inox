package jsonrpc

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	netaddr "github.com/inoxlang/inox/internal/netaddr"
	"github.com/inoxlang/inox/internal/reqratelimit"
	"github.com/rs/zerolog"
)

var (
	DEFAULT_METHOD_RATE_LIMITS  = []int{5, 20, 80} //rate limits for a single method
	DEFAULT_MESSAGE_RATE_LIMITS = [2]int{70, 200}
)

type rateLimiter struct {
	methodToLimiter map[string]*methodRateLimiter
	messageWindows  [2]reqratelimit.IWindow //1s and 10s windows.
	logger          zerolog.Logger
	lock            sync.Mutex

	config rateLimiterConfig
}

type rateLimiterConfig struct {
	methodRateLimits  []int  //defaults to DEFAULT_METHOD_RATE_LIMITS
	messageRateLimits [2]int //defaults to DEFAULT_INCOMING_MESSAGE_RATE_LIMITS
}

func newRateLimiter(logger zerolog.Logger, config rateLimiterConfig) *rateLimiter {

	if config.messageRateLimits == [2]int{} {
		config.messageRateLimits = DEFAULT_MESSAGE_RATE_LIMITS
	}

	if len(config.methodRateLimits) == 0 {
		config.methodRateLimits = DEFAULT_METHOD_RATE_LIMITS
	}

	r := &rateLimiter{
		methodToLimiter: make(map[string]*methodRateLimiter, 0),
		logger:          logger,
		config:          config,
		messageWindows: [2]reqratelimit.IWindow{
			reqratelimit.NewWindow(reqratelimit.WindowParameters{
				Duration:     time.Second,
				RequestCount: config.messageRateLimits[0],
			}),
			reqratelimit.NewWindow(reqratelimit.WindowParameters{
				Duration:     time.Second * 10,
				RequestCount: config.messageRateLimits[1],
			}),
		},
	}
	return r
}

// limits tell whether the request/message should be rate limited ($rateLimited result), $methodRateLimited is true if the limiting is caused
// by too many invocations of the same method.
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

	reqInfo := reqratelimit.WindowRequestInfo{
		Id:                reqId,
		Method:            info.Name,
		CreationTime:      now,
		RemoteAddrAndPort: addrPort,
		RemoteIpAddr:      addrPort.RemoteIp(),
		SentBytes:         0,
	}

	//Rate limit messages, regardless of the method.
	for i, window := range r.messageWindows {
		if !window.AllowRequest(reqInfo, r.logger) {
			//We don't break because all windows should be informed about the request.
			rateLimited = true
			if info.Name == "169" {
				fmt.Println("XX", i)
			}
		}
	}

	if rateLimited {
		return
	}

	rateLimited = false

	//Always allow first method invocation.
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

	//Create windows for the method if necessary.
	if len(methodLimiter.windows) == 0 {
		rateLimits := info.RateLimits
		if len(info.RateLimits) == 0 {
			rateLimits = DEFAULT_METHOD_RATE_LIMITS
		}

		windowDuration := time.Second //scaled x10 each time

		for _, reqCount := range rateLimits {
			if reqCount == 0 {
				methodLimiter.windows = append(methodLimiter.windows, nil)
			} else {
				methodLimiter.windows = append(methodLimiter.windows, reqratelimit.NewWindow(reqratelimit.WindowParameters{
					Duration:     windowDuration,
					RequestCount: reqCount,
				}))
			}

			windowDuration *= 10
		}
	}

	for _, window := range methodLimiter.windows {
		if window == nil {
			continue
		}

		if !window.AllowRequest(reqInfo, r.logger) {
			methodRateLimited = true
			//we don't break because all windows should be informed about the request.
		}
	}

	rateLimited = methodRateLimited
	return
}

type methodRateLimiter struct {
	info               MethodInfo
	lastInvocationTime atomic.Value
	windows            []reqratelimit.IWindow
}

func (l *methodRateLimiter) LastInvocationTime() time.Time {
	return l.lastInvocationTime.Load().(time.Time)
}
