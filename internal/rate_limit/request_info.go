package ratelimit

import (
	"time"

	nettypes "github.com/inoxlang/inox/internal/net_types"
	"github.com/oklog/ulid/v2"
)

type SlidingWindowRequestInfo struct {
	ULID              ulid.ULID //should not be used to retrieve time of request
	ULIDString        string
	Method            string
	CreationTime      time.Time
	RemoteAddrAndPort nettypes.RemoteAddrWithPort
	RemoteIpAddr      nettypes.RemoteIpAddr
	SentBytes         int
}

func (info SlidingWindowRequestInfo) IsMutation() bool {
	return info.Method == "POST" || info.Method == "PATCH" || info.Method == "DELETE"
}
