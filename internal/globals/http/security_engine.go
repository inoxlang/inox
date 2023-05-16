package internal

import (
	"sync"
	"time"

	cmap "github.com/orcaman/concurrent-map/v2"
)

const (
	SHARED_READ_BUST_WINDOW      = 10 * time.Second
	SHARED_READ_BURST_WINDOW_REQ = 60

	SHARED_WRITE_BURST_WINDOW     = 10 * time.Second
	SHARED_WRITE_BURST_WINDOW_REQ = 6
)

// the security engine is responsible for IP blacklisting, rate limiting & catpcha verification.
type securityEngine struct {
	mutex                  sync.Mutex
	readSlidingWindows     cmap.ConcurrentMap[RemoteAddrAndPort, *rateLimitingSlidingWindow]
	mutationSlidingWindows cmap.ConcurrentMap[RemoteAddrAndPort, *rateLimitingSlidingWindow]

	ipMitigationData cmap.ConcurrentMap[RemoteIpAddr, *remoteIpData]
	//hcaptchaSecret          string
	//captchaValidationClient *http.Client
}

func newSecurityEngine() *securityEngine {
	return &securityEngine{
		readSlidingWindows:     cmap.NewStringer[RemoteAddrAndPort, *rateLimitingSlidingWindow](),
		mutationSlidingWindows: cmap.NewStringer[RemoteAddrAndPort, *rateLimitingSlidingWindow](),
		ipMitigationData:       cmap.NewStringer[RemoteIpAddr, *remoteIpData](),
	}
}

func (engine *securityEngine) rateLimitRequest(req *HttpRequest, rw *HttpResponseWriter) bool {
	slidingWindow, windowReqInfo := engine.getSocketMitigationData(req)

	return !slidingWindow.allowRequest(windowReqInfo)
}

func (engine *securityEngine) getSocketMitigationData(req *HttpRequest) (*rateLimitingSlidingWindow, slidingWindowRequestInfo) {

	slidingWindowReqInfo := slidingWindowRequestInfo{
		ulid:              req.ULID,
		method:            string(req.Method),
		creationTime:      req.CreationTime,
		remoteAddrAndPort: req.RemoteAddrAndPort,
		remoteIpAddr:      req.RemoteIpAddr,
	}

	var slidingWindowMap cmap.ConcurrentMap[RemoteAddrAndPort, *rateLimitingSlidingWindow]
	var maxReqCount int

	if slidingWindowReqInfo.IsMutation() {
		maxReqCount = 2
		slidingWindowMap = engine.mutationSlidingWindows
	} else {
		slidingWindowMap = engine.readSlidingWindows
		maxReqCount = 10
	}

	ipLevelMigitigationData := engine.getIpLevelMitigationData(req)

	slidingWindow, present := slidingWindowMap.Get(req.RemoteAddrAndPort)
	if !present {
		slidingWindow = newRateLimitingSlidingWindow(rateLimitingWindowParameters{
			duration:     10 * time.Second,
			requestCount: maxReqCount,
		})
		if slidingWindowReqInfo.IsMutation() {
			slidingWindow.burstWindow = ipLevelMigitigationData.sharedWriteBurstWindow
		} else {
			slidingWindow.burstWindow = ipLevelMigitigationData.sharedReadBurstWindow
		}
		slidingWindowMap.Set(req.RemoteAddrAndPort, slidingWindow)
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
