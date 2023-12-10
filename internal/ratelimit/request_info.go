package ratelimit

import (
	"time"

	netaddr "github.com/inoxlang/inox/internal/netaddr"
)

type SlidingWindowRequestInfo struct {
	Id                string
	Method            string
	CreationTime      time.Time
	RemoteAddrAndPort netaddr.RemoteAddrWithPort
	RemoteIpAddr      netaddr.RemoteIpAddr
	SentBytes         int
}
