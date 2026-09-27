package device

import "net"

// LocalLANIP returns this machine's LAN-reachable IPv4 address (e.g.
// "192.168.1.25"), or false if none is found. Used to default the Metro
// bundler host for a physical device: "localhost" from a phone's network
// stack refers to the phone itself, not this Mac, so a real device must be
// pointed at this machine's actual address on the network.
//
// A standard private-LAN address (192.168.*, 10.*, 172.16-31.*) is preferred
// over any other assigned address (e.g. a VPN's point-to-point IP), since a
// phone on the same WiFi network can reach the former but usually not the
// latter.
func LocalLANIP() (string, bool) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "", false
	}
	var fallback string
	for _, addr := range addrs {
		ipNet, ok := addr.(*net.IPNet)
		if !ok || ipNet.IP.IsLoopback() || ipNet.IP.IsLinkLocalUnicast() {
			continue
		}
		ip4 := ipNet.IP.To4()
		if ip4 == nil {
			continue
		}
		if isPrivateLANAddr(ip4) {
			return ip4.String(), true
		}
		if fallback == "" {
			fallback = ip4.String()
		}
	}
	if fallback != "" {
		return fallback, true
	}
	return "", false
}

func isPrivateLANAddr(ip net.IP) bool {
	return ip[0] == 192 && ip[1] == 168 ||
		ip[0] == 10 ||
		(ip[0] == 172 && ip[1] >= 16 && ip[1] <= 31)
}
