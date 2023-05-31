package http_ns

import (
	"log"
	"sync"
	"time"

	"github.com/inoxlang/inox/internal/utils"
	"github.com/oklog/ulid/v2"
	"github.com/rs/zerolog"
)

const (
	MAX_SOCKET_SHARE_OF_SHARED_WINDOW = 0.50
)

type irateLimitingWindow interface {
	allowRequest(rInfo slidingWindowRequestInfo, logger zerolog.Logger) (ok bool)
	//enrichRequestAfterHandling(reqInfo *IncomingRequestInfo)
}

type rateLimitingWindowParameters struct {
	duration     time.Duration
	requestCount int
}

type rateLimitingSlidingWindow struct {
	duration      time.Duration
	requests      []slidingWindowRequestInfo
	ipLevelWindow irateLimitingWindow
	mutex         sync.Mutex
}

type slidingWindowRequestInfo struct {
	ulid              ulid.ULID //should not be used to retrieve time of request
	ulidString        string
	method            string
	creationTime      time.Time
	remoteAddrAndPort RemoteAddrAndPort
	remoteIpAddr      RemoteIpAddr
	sentBytes         int
}

func (info slidingWindowRequestInfo) IsMutation() bool {
	return info.method == "POST" || info.method == "PATCH" || info.method == "DELETE"
}

func newRateLimitingSlidingWindow(params rateLimitingWindowParameters) *rateLimitingSlidingWindow {

	if params.requestCount <= 0 {
		log.Panicln("cannot create sliding window with request count less or equal to zero")
	}

	window := &rateLimitingSlidingWindow{
		duration:      params.duration,
		requests:      make([]slidingWindowRequestInfo, params.requestCount),
		ipLevelWindow: nil,
	}

	for i := range window.requests {
		window.requests[i].ulid = ulid.ULID{}
	}

	return window
}

// TODO: treat many HTTP/1.1 connections from same IP as suspicious
func (window *rateLimitingSlidingWindow) allowRequest(rInfo slidingWindowRequestInfo, logger zerolog.Logger) (ok bool) {
	window.mutex.Lock()
	defer window.mutex.Unlock()
	candidateSlotIndexes := make([]int, 0)

	//if we find an empty slot for the request we accept it immediately
	//otherwise we search for slots that contain "old" requests.
	for i, req := range window.requests {

		if req.ulid == (ulid.ULID{}) { //empty slot
			window.requests[i] = rInfo
			logger.Debug().Msg("found empty slot for request" + req.ulidString)
			return true
		}

		if rInfo.creationTime.Sub(req.creationTime) > window.duration {
			candidateSlotIndexes = append(candidateSlotIndexes, i)
		}
	}

	logger.Log().Str(REQUEST_ID_LOG_FIELD_NAME, rInfo.ulidString).Int("candidateSlots", len(candidateSlotIndexes))

	switch len(candidateSlotIndexes) {
	case 0:
		//find the oldest slot and store the new request
		oldestRequestTime := window.requests[0].creationTime
		oldestRequestSlotIndex := 0
		for i, req := range window.requests {
			if req.creationTime.Before(oldestRequestTime) {
				oldestRequestTime = req.creationTime
				oldestRequestSlotIndex = i
				break
			}
		}

		window.requests[oldestRequestSlotIndex] = rInfo

		timeSinceOldestRequest := rInfo.creationTime.Sub(oldestRequestTime)
		//burst
		if timeSinceOldestRequest < window.duration/2 {
			return false
		}

		return window.ipLevelWindow != nil && window.ipLevelWindow.allowRequest(rInfo, logger)
	case 1:
		window.requests[candidateSlotIndexes[0]] = rInfo
		return true
	default:
		oldestRequestTime := window.requests[candidateSlotIndexes[0]].creationTime
		oldestRequestSlotIndex := candidateSlotIndexes[0]
		for _, slotIndex := range candidateSlotIndexes[1:] {
			requestTime := window.requests[slotIndex].creationTime

			if requestTime.Before(oldestRequestTime) {
				oldestRequestTime = requestTime
				oldestRequestSlotIndex = slotIndex
			}
		}

		window.requests[oldestRequestSlotIndex] = rInfo
		return true
	}
}

// sharedRateLimitingWindow is shared between several sockets.
type sharedRateLimitingWindow struct {
	*rateLimitingSlidingWindow
}

func newSharedRateLimitingWindow(params rateLimitingWindowParameters) *sharedRateLimitingWindow {
	if params.requestCount <= 0 {
		log.Panicln("cannot create sliding window with request count less or equal to zero")
	}

	window := &sharedRateLimitingWindow{
		newRateLimitingSlidingWindow(params),
	}

	return window
}

func (window *sharedRateLimitingWindow) allowRequest(req slidingWindowRequestInfo, logger zerolog.Logger) (ok bool) {
	//request count for the current socket
	prevReqCount := 0
	sockets := make([]RemoteAddrAndPort, 0)

	for _, windowReq := range window.rateLimitingSlidingWindow.requests {

		// ignore "old" requests
		if req.creationTime.Sub(windowReq.creationTime) >= window.duration {
			continue
		}

		if !utils.SliceContains(sockets, windowReq.remoteAddrAndPort) {
			sockets = append(sockets, windowReq.remoteAddrAndPort)
		}

		if windowReq.remoteAddrAndPort == req.remoteAddrAndPort {
			prevReqCount += 1
		}
	}

	//add current socket
	if !utils.SliceContains(sockets, req.remoteAddrAndPort) {
		sockets = append(sockets, req.remoteAddrAndPort)
	}

	reqCountF := float32(prevReqCount + 1)
	totalReqCountF := float32(len(window.requests))
	maxSocketReqCount := totalReqCountF / float32(len(sockets))

	//socket has exceeded its share
	ok = (len(sockets) == 1 && reqCountF < totalReqCountF*MAX_SOCKET_SHARE_OF_SHARED_WINDOW) ||
		(len(sockets) != 1 && reqCountF <= maxSocketReqCount)

	if !window.rateLimitingSlidingWindow.allowRequest(req, logger) {
		ok = false
	}

	return
}
