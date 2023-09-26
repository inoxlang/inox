package ratelimit

import (
	"time"

	nettypes "github.com/inoxlang/inox/internal/net_types"
)

type SlidingWindowRequestInfo struct {
	Id                string
	Method            string
	CreationTime      time.Time
	RemoteAddrAndPort nettypes.RemoteAddrWithPort
	RemoteIpAddr      nettypes.RemoteIpAddr
	SentBytes         int
}

func (info SlidingWindowRequestInfo) IsMutation() bool {
	return info.Method == "POST" || info.Method == "PATCH" || info.Method == "DELETE"
}
