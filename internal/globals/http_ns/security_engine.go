package http_ns

import (
	"sync"
	"time"

	netaddr "github.com/inoxlang/inox/internal/netaddr"
	"github.com/inoxlang/inox/internal/ratelimit"
	cmap "github.com/orcaman/concurrent-map/v2"
	"github.com/rs/zerolog"
)

const (
	//socket
	SOCKET_WINDOW              = 10 * time.Second
	SOCKET_MAX_READ_REQ_COUNT  = 10
	SOCKET_MAX_WRITE_REQ_COUNT = 2

	//ip level
	SHARED_READ_BUST_WINDOW      = 10 * time.Second
	SHARED_READ_BURST_WINDOW_REQ = 60

	SHARED_WRITE_BURST_WINDOW     = 10 * time.Second
	SHARED_WRITE_BURST_WINDOW_REQ = 6
)

// the security engine is responsible for IP blacklisting, rate limiting & catpcha verification.
type securityEngine struct {
	mutex                  sync.Mutex
	logger                 zerolog.Logger
	debugLogger            zerolog.Logger
	readSlidingWindows     cmap.ConcurrentMap[netaddr.RemoteAddrWithPort, *ratelimit.SlidingWindow]
	mutationSlidingWindows cmap.ConcurrentMap[netaddr.RemoteAddrWithPort, *ratelimit.SlidingWindow]

	ipMitigationData cmap.ConcurrentMap[netaddr.RemoteIpAddr, *remoteIpData]
	//hcaptchaSecret          string
	//captchaValidationClient *http.Client
}

func newSecurityEngine(logger zerolog.Logger) *securityEngine {

	return &securityEngine{
		logger:                 logger,
		debugLogger:            logger,
		readSlidingWindows:     cmap.NewStringer[netaddr.RemoteAddrWithPort, *ratelimit.SlidingWindow](),
		mutationSlidingWindows: cmap.NewStringer[netaddr.RemoteAddrWithPort, *ratelimit.SlidingWindow](),
		ipMitigationData:       cmap.NewStringer[netaddr.RemoteIpAddr, *remoteIpData](),
	}
}

func (engine *securityEngine) rateLimitRequest(req *HttpRequest, rw *HttpResponseWriter) bool {
	slidingWindow, windowReqInfo := engine.getSocketMitigationData(req)

	if !slidingWindow.AllowRequest(windowReqInfo, engine.debugLogger) {
		engine.logger.Log().Str("rateLimit", req.ULIDString)
		return true
	}

	return false
}

func (engine *securityEngine) getSocketMitigationData(req *HttpRequest) (*ratelimit.SlidingWindow, ratelimit.SlidingWindowRequestInfo) {
	slidingWindowReqInfo := ratelimit.SlidingWindowRequestInfo{
		Id:                req.ULIDString,
		Method:            string(req.Method),
		CreationTime:      req.CreationTime,
		RemoteAddrAndPort: req.RemoteAddrAndPort,
		RemoteIpAddr:      req.RemoteIpAddr,
	}

	var slidingWindowMap cmap.ConcurrentMap[netaddr.RemoteAddrWithPort, *ratelimit.SlidingWindow]
	var maxReqCount int

	if IsMutationMethod(slidingWindowReqInfo.Method) {
		maxReqCount = SOCKET_MAX_WRITE_REQ_COUNT
		slidingWindowMap = engine.mutationSlidingWindows
	} else {
		slidingWindowMap = engine.readSlidingWindows
		maxReqCount = SOCKET_MAX_READ_REQ_COUNT
	}

	ipLevelMigitigationData := engine.getIpLevelMitigationData(req)

	slidingWindow, present := slidingWindowMap.Get(req.RemoteAddrAndPort)
	if !present {
		engine.debugLogger.Debug().Str("newSlidingWindowFor", string(req.RemoteAddrAndPort)).Send()
		slidingWindow = ratelimit.NewSlidingWindow(ratelimit.WindowParameters{
			Duration:     SOCKET_WINDOW,
			RequestCount: maxReqCount,
		})
		if IsMutationMethod(slidingWindowReqInfo.Method) {
			slidingWindow.SetIpLevelWindow(ipLevelMigitigationData.sharedWriteBurstWindow)
		} else {
			slidingWindow.SetIpLevelWindow(ipLevelMigitigationData.sharedReadBurstWindow)
		}
		slidingWindowMap.Set(req.RemoteAddrAndPort, slidingWindow)
	} else {
		engine.debugLogger.Debug().Str("foundSlidingWindowFor", string(req.RemoteAddrAndPort)).Send()
	}

	return slidingWindow, slidingWindowReqInfo
}

func (engine *securityEngine) getIpLevelMitigationData(req *HttpRequest) *remoteIpData {
	if _mitigationData, found := engine.ipMitigationData.Get(req.RemoteIpAddr); found {
		return _mitigationData
	}

	//else create data

	engine.mutex.Lock()
	defer engine.mutex.Unlock()

	mitigationData := &remoteIpData{
		persistedRemoteIpData: persistedRemoteIpData{
			ip:                   req.RemoteIpAddr,
			respStatusCodeCounts: make(map[int]int),
		},
	}

	mitigationData.sharedReadBurstWindow = ratelimit.NewSharedRateLimitingWindow(ratelimit.WindowParameters{
		Duration:     SHARED_READ_BUST_WINDOW,
		RequestCount: SHARED_READ_BURST_WINDOW_REQ,
	})
	mitigationData.sharedWriteBurstWindow = ratelimit.NewSharedRateLimitingWindow(ratelimit.WindowParameters{
		Duration:     SHARED_WRITE_BURST_WINDOW,
		RequestCount: SHARED_WRITE_BURST_WINDOW_REQ,
	})

	engine.ipMitigationData.Set(req.RemoteIpAddr, mitigationData)
	return mitigationData
}

func (engine *securityEngine) postHandle(req *HttpRequest, rw *HttpResponseWriter) {
	mitigationData := engine.getIpLevelMitigationData(req)
	slidingWindow, _ := engine.getSocketMitigationData(req)

	status := rw.Status()

	//TODO:

	_ = mitigationData
	_ = slidingWindow
	_ = status
}
