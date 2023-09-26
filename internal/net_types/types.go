package nettypes

import "net"

type RemoteAddrWithPort string

func (s RemoteAddrWithPort) String() string {
	return string(s)
}

func (s RemoteAddrWithPort) RemoteIp() RemoteIpAddr {
	ip, _, err := net.SplitHostPort(string(s))
	if err != nil {
		panic(err)
	}
	return RemoteIpAddr(ip)
}

type RemoteIpAddr string

func (s RemoteIpAddr) String() string {
	return string(s)
}
