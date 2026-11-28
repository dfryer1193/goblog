package stats

import (
	"net"
	"net/http"
	"net/netip"
	"strings"
)

// clientIP is the IP address of the client that sent r: the rightmost
// X-Forwarded-For entry, which the gateway (the only proxy in front of
// goblog) appended, or the TCP peer without one. It's used for
// classification, the Visitor ID and lookups, and never emitted or logged.
// It's invalid if neither is an IP address.
func clientIP(r *http.Request) netip.Addr {
	if values := r.Header.Values("X-Forwarded-For"); len(values) > 0 {
		entries := strings.Split(values[len(values)-1], ",")
		if ip, err := netip.ParseAddr(strings.TrimSpace(entries[len(entries)-1])); err == nil {
			return ip.Unmap()
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip, err := netip.ParseAddr(host)
	if err != nil {
		return netip.Addr{}
	}
	return ip.Unmap()
}
