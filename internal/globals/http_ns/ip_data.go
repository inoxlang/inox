package http_ns

import (
	"sync"

	netaddr "github.com/inoxlang/inox/internal/netaddr"
	"github.com/inoxlang/inox/internal/reqratelimit"
)

type remoteIpData struct {
	persistedRemoteIpData

	mutex sync.Mutex
	//resourceDataMap                         concmap.ConcurrentMap
	currentCaptchProtectedPostResourcePaths []string
	sharedReadBurstWindow                   reqratelimit.IWindow
	sharedWriteBurstWindow                  reqratelimit.IWindow
	isBlackListed                           bool
}

type persistedRemoteIpData struct {
	ip                   netaddr.RemoteIpAddr
	respStatusCodeCounts map[int]int
}
