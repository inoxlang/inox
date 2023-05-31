package http_ns

import (
	"sync"
	"time"

	core "github.com/inoxlang/inox/internal/core"
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
	readSlidingWindows     cmap.ConcurrentMap[RemoteAddrAndPort, *rateLimitingSlidingWindow]
	mutationSlidingWindows cmap.ConcurrentMap[RemoteAddrAndPort, *rateLimitingSlidingWindow]

	ipMitigationData cmap.ConcurrentMap[RemoteIpAddr, *remoteIpData]
	//hcaptchaSecret          string
	//captchaValidationClient *http.Client
}

func newSecurityEngine(baseLogger zerolog.Logger, serverLogSrc string) *securityEngine {
	logger := baseLogger.With().Str(core.SOURCE_LOG_FIELD_NAME, serverLogSrc+"/sec").Logger()

	return &securityEngine{
		logger:                 logger,
		debugLogger:            logger,
		readSlidingWindows:     cmap.NewStringer[RemoteAddrAndPort, *rateLimitingSlidingWindow](),
		mutationSlidingWindows: cmap.NewStringer[RemoteAddrAndPort, *rateLimitingSlidingWindow](),
		ipMitigationData:       cmap.NewStringer[RemoteIpAddr, *remoteIpData](),
	}
}

func (engine *securityEngine) rateLimitRequest(req *HttpRequest, rw *HttpResponseWriter) bool {
	slidingWindow, windowReqInfo := engine.getSocketMitigationData(req)

	if !slidingWindow.allowRequest(windowReqInfo, engine.debugLogger) {
		engine.logger.Log().Str("rateLimit", req.ULIDString)
		return true
	}

	return false
}

func (engine *securityEngine) getSocketMitigationData(req *HttpRequest) (*rateLimitingSlidingWindow, slidingWindowRequestInfo) {
	slidingWindowReqInfo := slidingWindowRequestInfo{
		ulid:              req.ULID,
		ulidString:        req.ULIDString,
		method:            string(req.Method),
		creationTime:      req.CreationTime,
		remoteAddrAndPort: req.RemoteAddrAndPort,
		remoteIpAddr:      req.RemoteIpAddr,
	}

	var slidingWindowMap cmap.ConcurrentMap[RemoteAddrAndPort, *rateLimitingSlidingWindow]
	var maxReqCount int

	if slidingWindowReqInfo.IsMutation() {
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
		slidingWindow = newRateLimitingSlidingWindow(rateLimitingWindowParameters{
			duration:     SOCKET_WINDOW,
			requestCount: maxReqCount,
		})
		if slidingWindowReqInfo.IsMutation() {
			slidingWindow.ipLevelWindow = ipLevelMigitigationData.sharedWriteBurstWindow
		} else {
			slidingWindow.ipLevelWindow = ipLevelMigitigationData.sharedReadBurstWindow
		}
		slidingWindowMap.Set(req.RemoteAddrAndPort, slidingWindow)
	} else {
		engine.debugLogger.Log().Str("foundSlidingWindowFor", string(req.RemoteAddrAndPort)).Send()
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

	mitigationData.sharedReadBurstWindow = newSharedRateLimitingWindow(rateLimitingWindowParameters{
		duration:     SHARED_READ_BUST_WINDOW,
		requestCount: SHARED_READ_BURST_WINDOW_REQ,
	})
	mitigationData.sharedWriteBurstWindow = newSharedRateLimitingWindow(rateLimitingWindowParameters{
		duration:     SHARED_WRITE_BURST_WINDOW,
		requestCount: SHARED_WRITE_BURST_WINDOW_REQ,
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
