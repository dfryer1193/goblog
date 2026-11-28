package stats

import (
	"fmt"
	"log/slog"
	"net/netip"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
)

// excludedNetworks decides whether a client IP is in the StatsConfig's
// excluded_networks. Parsing the CIDRs is cached, keyed by the list's value,
// so a request only reparses them when the list has changed.
type excludedNetworks struct {
	logger *slog.Logger
	// parsed is the list last parsed. It's replaced, never changed.
	parsed atomic.Pointer[parsedNetworks]

	// mu serialises reparsing and guards warned.
	mu sync.Mutex
	// warned are the invalid entries a warning has been logged for, so each
	// is warned about once, not on every request.
	warned map[string]bool
}

// parsedNetworks is one value of excluded_networks and its parsed CIDRs.
type parsedNetworks struct {
	raw      []string
	prefixes []netip.Prefix
}

// newExcludedNetworks starts with no excluded networks, and logs warnings
// about invalid entries to logger.
func newExcludedNetworks(logger *slog.Logger) *excludedNetworks {
	e := &excludedNetworks{logger: logger, warned: map[string]bool{}}
	e.parsed.Store(&parsedNetworks{})
	return e
}

// contains reports whether ip is in one of the networks in raw, the current
// value of excluded_networks. Entries that aren't CIDRs are skipped.
func (e *excludedNetworks) contains(raw []string, ip netip.Addr) bool {
	if !ip.IsValid() {
		return false
	}
	for _, p := range e.prefixesOf(raw) {
		if p.Contains(ip) {
			return true
		}
	}
	return false
}

// prefixesOf is raw's valid CIDRs, parsed once for each new value of raw.
func (e *excludedNetworks) prefixesOf(raw []string) []netip.Prefix {
	if p := e.parsed.Load(); slices.Equal(p.raw, raw) {
		return p.prefixes
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if p := e.parsed.Load(); slices.Equal(p.raw, raw) {
		return p.prefixes
	}
	p := &parsedNetworks{raw: slices.Clone(raw)}
	for i, entry := range raw {
		prefix, err := netip.ParsePrefix(strings.TrimSpace(entry))
		if err != nil {
			// The warning says which entry it is by its position, never
			// its text, which may be the author's own network.
			if !e.warned[entry] {
				e.warned[entry] = true
				e.logger.Warn(fmt.Sprintf("excluded_networks entry %d isn't a valid CIDR: %s; skipping it",
					i+1, whyNotCIDR(entry)))
			}
			continue
		}
		p.prefixes = append(p.prefixes, prefix.Masked())
	}
	e.parsed.Store(p)
	return p.prefixes
}

// whyNotCIDR says why entry, which [netip.ParsePrefix] rejected, isn't a CIDR,
// without saying what's in it.
func whyNotCIDR(entry string) string {
	addr, bits, ok := strings.Cut(strings.TrimSpace(entry), "/")
	switch {
	case !ok:
		return "it has no /prefix length"
	case !isAddr(addr):
		return "its address isn't an IP address"
	case bits == "":
		return "its prefix length is empty"
	default:
		return "its prefix length isn't one for its address"
	}
}

// isAddr reports whether s is an IP address, without a zone (which a CIDR
// can't have).
func isAddr(s string) bool {
	ip, err := netip.ParseAddr(s)
	return err == nil && ip.Zone() == ""
}
