package http_ns

import (
	"sync"

	nettypes "github.com/inoxlang/inox/internal/net_types"
	ratelimit "github.com/inoxlang/inox/internal/rate_limit"
)

type remoteIpData struct {
	persistedRemoteIpData

	mutex sync.Mutex
	//resourceDataMap                         concmap.ConcurrentMap
	currentCaptchProtectedPostResourcePaths []string
	sharedReadBurstWindow                   ratelimit.IWindow
	sharedWriteBurstWindow                  ratelimit.IWindow
	isBlackListed                           bool
}

type persistedRemoteIpData struct {
	ip                   nettypes.RemoteIpAddr
	respStatusCodeCounts map[int]int
}
