package http_ns

import (
	"sync"
	"time"

	netaddr "github.com/inoxlang/inox/internal/netaddr"
	"github.com/inoxlang/inox/internal/reqratelimit"
	cmap "github.com/orcaman/concurrent-map/v2"
	"github.com/rs/zerolog"
)

const (
	//socket
	SOCKET_RLIMIT_WINDOW       = 10 * time.Second
	SOCKET_MAX_READ_REQ_COUNT  = 10
	SOCKET_MAX_WRITE_REQ_COUNT = 5

	//ip level
	SHARED_READ_BURST_WINDOW     = 10 * time.Second
	SHARED_READ_BURST_WINDOW_REQ = 60

	SHARED_WRITE_BURST_WINDOW     = 10 * time.Second
	SHARED_WRITE_BURST_WINDOW_REQ = 10
)

// the security engine is responsible for IP blacklisting, rate limiting & catpcha verification.
type securityEngine struct {
	mutex           sync.Mutex
	logger          zerolog.Logger
	debugLogger     zerolog.Logger
	readWindows     cmap.ConcurrentMap[netaddr.RemoteAddrWithPort, *reqratelimit.Window]
	mutationWindows cmap.ConcurrentMap[netaddr.RemoteAddrWithPort, *reqratelimit.Window]

	ipMitigationData cmap.ConcurrentMap[netaddr.RemoteIpAddr, *remoteIpData]
	//hcaptchaSecret          string
	//captchaValidationClient *http.Client
}

func newSecurityEngine(logger zerolog.Logger) *securityEngine {

	return &securityEngine{
		logger:           logger,
		debugLogger:      logger,
		readWindows:      cmap.NewStringer[netaddr.RemoteAddrWithPort, *reqratelimit.Window](),
		mutationWindows:  cmap.NewStringer[netaddr.RemoteAddrWithPort, *reqratelimit.Window](),
		ipMitigationData: cmap.NewStringer[netaddr.RemoteIpAddr, *remoteIpData](),
	}
}

func (engine *securityEngine) rateLimitRequest(req *Request, rw *ResponseWriter) bool {
	window, windowReqInfo := engine.getSocketMitigationData(req)

	if !window.AllowRequest(windowReqInfo, engine.debugLogger) {
		engine.logger.Log().Str("rateLimit", req.ULIDString)
		return true
	}

	return false
}

func (engine *securityEngine) getSocketMitigationData(req *Request) (*reqratelimit.Window, reqratelimit.WindowRequestInfo) {
	windowReqInfo := reqratelimit.WindowRequestInfo{
		Id:                req.ULIDString,
		Method:            string(req.Method),
		CreationTime:      req.CreationTime,
		RemoteAddrAndPort: req.RemoteAddrAndPort,
		RemoteIpAddr:      req.RemoteIpAddr,
	}

	var windowMap cmap.ConcurrentMap[netaddr.RemoteAddrWithPort, *reqratelimit.Window]
	var maxReqCount int

	if IsMutationMethod(windowReqInfo.Method) {
		maxReqCount = SOCKET_MAX_WRITE_REQ_COUNT
		windowMap = engine.mutationWindows
	} else {
		windowMap = engine.readWindows
		maxReqCount = SOCKET_MAX_READ_REQ_COUNT
	}

	ipLevelMigitigationData := engine.getIpLevelMitigationData(req)

	window, present := windowMap.Get(req.RemoteAddrAndPort)
	if !present {
		engine.debugLogger.Debug().Str("newWindowFor", string(req.RemoteAddrAndPort)).Send()
		window = reqratelimit.NewWindow(reqratelimit.WindowParameters{
			Duration:     SOCKET_RLIMIT_WINDOW,
			RequestCount: maxReqCount,
		})
		if IsMutationMethod(windowReqInfo.Method) {
			window.SetIpLevelWindow(ipLevelMigitigationData.sharedWriteBurstWindow)
		} else {
			window.SetIpLevelWindow(ipLevelMigitigationData.sharedReadBurstWindow)
		}
		windowMap.Set(req.RemoteAddrAndPort, window)
	} else {
		engine.debugLogger.Debug().Str("foundWindowFor", string(req.RemoteAddrAndPort)).Send()
	}

	return window, windowReqInfo
}

func (engine *securityEngine) getIpLevelMitigationData(req *Request) *remoteIpData {
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

	mitigationData.sharedReadBurstWindow = reqratelimit.NewSharedRateLimitingWindow(reqratelimit.WindowParameters{
		Duration:     SHARED_READ_BURST_WINDOW,
		RequestCount: SHARED_READ_BURST_WINDOW_REQ,
	})
	mitigationData.sharedWriteBurstWindow = reqratelimit.NewSharedRateLimitingWindow(reqratelimit.WindowParameters{
		Duration:     SHARED_WRITE_BURST_WINDOW,
		RequestCount: SHARED_WRITE_BURST_WINDOW_REQ,
	})

	engine.ipMitigationData.Set(req.RemoteIpAddr, mitigationData)
	return mitigationData
}

func (engine *securityEngine) postHandle(req *Request, rw *ResponseWriter) {
	mitigationData := engine.getIpLevelMitigationData(req)
	window, _ := engine.getSocketMitigationData(req)

	status := rw.SentStatus()

	//TODO:

	_ = mitigationData
	_ = window
	_ = status
}
