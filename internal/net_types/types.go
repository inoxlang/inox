package nettypes

import (
	"errors"
	"net"
)

type RemoteAddrWithPort string

func RemoteAddrWithPortFrom(s string) (RemoteAddrWithPort, error) {
	if s == "" {
		return "", errors.New("empty string")
	}
	_, _, err := net.SplitHostPort(string(s))
	if err != nil {
		return "", err
	}
	return RemoteAddrWithPort(s), nil
}

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
