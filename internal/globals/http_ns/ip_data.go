package http_ns

import "sync"

type remoteIpData struct {
	persistedRemoteIpData

	mutex sync.Mutex
	//resourceDataMap                         concmap.ConcurrentMap
	currentCaptchProtectedPostResourcePaths []string
	sharedReadBurstWindow                   irateLimitingWindow
	sharedWriteBurstWindow                  irateLimitingWindow
	isBlackListed                           bool
}

type persistedRemoteIpData struct {
	ip                   RemoteIpAddr
	respStatusCodeCounts map[int]int
}
