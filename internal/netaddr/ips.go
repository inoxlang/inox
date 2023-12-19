package netaddr

import "net"

func FilterInterfaceIPs(filter func(ipnet *net.IPNet) bool) ([]net.IP, error) {
	var ips []net.IP
	addresses, err := net.InterfaceAddrs()
	if err != nil {
		return nil, err
	}

	for _, addr := range addresses {
		if ipnet, ok := addr.(*net.IPNet); ok && filter(ipnet) {
			if ipnet.IP.To16() != nil /*IPv4 or IPv6*/ {
				ips = append(ips, ipnet.IP)
			}
		}
	}
	return ips, nil
}

func GetGlobalUnicastIPs() ([]net.IP, error) {
	return FilterInterfaceIPs(func(ipnet *net.IPNet) bool {
		return ipnet.IP.IsGlobalUnicast()
	})
}

func GetPrivateIPs() ([]net.IP, error) {
	return FilterInterfaceIPs(func(ipnet *net.IPNet) bool {
		return ipnet.IP.IsPrivate()
	})
}
