package stats

import (
	"net"
	"net/netip"
	"strings"
)

// clientIP is the IP address of the client that sent req: the rightmost
// X-Forwarded-For entry, which the gateway (the only proxy in front of
// goblog) appended, or the TCP peer without one. It's used for
// classification, the Visitor ID and lookups, and never emitted or logged.
// It's invalid if neither is an IP address.
func (req request) clientIP() netip.Addr {
	if req.forwardedFor != "" {
		entries := strings.Split(req.forwardedFor, ",")
		if ip, err := netip.ParseAddr(strings.TrimSpace(entries[len(entries)-1])); err == nil {
			return ip.Unmap()
		}
	}
	host, _, err := net.SplitHostPort(req.remoteAddr)
	if err != nil {
		host = req.remoteAddr
	}
	ip, err := netip.ParseAddr(host)
	if err != nil {
		return netip.Addr{}
	}
	return ip.Unmap()
}
