package ratelimit

import (
	"log"
	"sync"
	"time"

	nettypes "github.com/inoxlang/inox/internal/net_types"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/rs/zerolog"
)

const (
	MAX_SOCKET_SHARE_OF_SHARED_WINDOW = 0.50
	REQUEST_ID_LOG_FIELD_NAME         = "reqID"
)

type IWindow interface {
	AllowRequest(rInfo SlidingWindowRequestInfo, logger zerolog.Logger) (ok bool)
	//enrichRequestAfterHandling(reqInfo *IncomingRequestInfo)
}

type WindowParameters struct {
	Duration     time.Duration
	RequestCount int
}

type SlidingWindow struct {
	duration      time.Duration
	requests      []SlidingWindowRequestInfo
	ipLevelWindow IWindow
	mutex         sync.Mutex
}

func NewSlidingWindow(params WindowParameters) *SlidingWindow {

	if params.RequestCount <= 0 {
		log.Panicln("cannot create sliding window with request count less or equal to zero")
	}

	window := &SlidingWindow{
		duration:      params.Duration,
		requests:      make([]SlidingWindowRequestInfo, params.RequestCount),
		ipLevelWindow: nil,
	}

	return window
}

func (w *SlidingWindow) SetIpLevelWindow(window IWindow) {
	w.ipLevelWindow = window
}

// TODO: treat many HTTP/1.1 connections from same IP as suspicious
func (window *SlidingWindow) AllowRequest(rInfo SlidingWindowRequestInfo, logger zerolog.Logger) (ok bool) {
	window.mutex.Lock()
	defer window.mutex.Unlock()
	candidateSlotIndexes := make([]int, 0)

	//if we find an empty slot for the request we accept it immediately
	//otherwise we search for slots that contain "old" requests.
	for i, req := range window.requests {

		if req.Id == "" { //empty slot
			window.requests[i] = rInfo
			logger.Debug().Msg("found empty slot for request" + req.Id)
			return true
		}

		if rInfo.CreationTime.Sub(req.CreationTime) > window.duration {
			candidateSlotIndexes = append(candidateSlotIndexes, i)
		}
	}

	logger.Log().Str(REQUEST_ID_LOG_FIELD_NAME, rInfo.Id).Int("candidateSlots", len(candidateSlotIndexes))

	switch len(candidateSlotIndexes) {
	case 0:
		//find the oldest slot and store the new request
		oldestRequestTime := window.requests[0].CreationTime
		oldestRequestSlotIndex := 0
		for i, req := range window.requests {
			if req.CreationTime.Before(oldestRequestTime) {
				oldestRequestTime = req.CreationTime
				oldestRequestSlotIndex = i
				break
			}
		}

		window.requests[oldestRequestSlotIndex] = rInfo

		timeSinceOldestRequest := rInfo.CreationTime.Sub(oldestRequestTime)
		//burst
		if timeSinceOldestRequest < window.duration/2 {
			return false
		}

		return window.ipLevelWindow != nil && window.ipLevelWindow.AllowRequest(rInfo, logger)
	case 1:
		window.requests[candidateSlotIndexes[0]] = rInfo
		return true
	default:
		oldestRequestTime := window.requests[candidateSlotIndexes[0]].CreationTime
		oldestRequestSlotIndex := candidateSlotIndexes[0]
		for _, slotIndex := range candidateSlotIndexes[1:] {
			requestTime := window.requests[slotIndex].CreationTime

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
	*SlidingWindow
}

func NewSharedRateLimitingWindow(params WindowParameters) *sharedRateLimitingWindow {
	if params.RequestCount <= 0 {
		log.Panicln("cannot create sliding window with request count less or equal to zero")
	}

	window := &sharedRateLimitingWindow{
		NewSlidingWindow(params),
	}

	return window
}

func (window *sharedRateLimitingWindow) AllowRequest(req SlidingWindowRequestInfo, logger zerolog.Logger) (ok bool) {
	//request count for the current socket
	prevReqCount := 0
	sockets := make([]nettypes.RemoteAddrWithPort, 0)

	for _, windowReq := range window.SlidingWindow.requests {

		// ignore "old" requests
		if req.CreationTime.Sub(windowReq.CreationTime) >= window.duration {
			continue
		}

		if !utils.SliceContains(sockets, windowReq.RemoteAddrAndPort) {
			sockets = append(sockets, windowReq.RemoteAddrAndPort)
		}

		if windowReq.RemoteAddrAndPort == req.RemoteAddrAndPort {
			prevReqCount += 1
		}
	}

	//add current socket
	if !utils.SliceContains(sockets, req.RemoteAddrAndPort) {
		sockets = append(sockets, req.RemoteAddrAndPort)
	}

	reqCountF := float32(prevReqCount + 1)
	totalReqCountF := float32(len(window.requests))
	maxSocketReqCount := totalReqCountF / float32(len(sockets))

	//socket has exceeded its share
	ok = (len(sockets) == 1 && reqCountF < totalReqCountF*MAX_SOCKET_SHARE_OF_SHARED_WINDOW) ||
		(len(sockets) != 1 && reqCountF <= maxSocketReqCount)

	if !window.SlidingWindow.AllowRequest(req, logger) {
		ok = false
	}

	return
}
