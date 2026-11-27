package stats

import (
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
	for _, entry := range raw {
		prefix, err := netip.ParsePrefix(strings.TrimSpace(entry))
		if err != nil {
			if !e.warned[entry] {
				e.warned[entry] = true
				e.logger.Warn("Skipping an excluded network that isn't a CIDR", "entry", entry, "error", err)
			}
			continue
		}
		p.prefixes = append(p.prefixes, prefix.Masked())
	}
	e.parsed.Store(p)
	return p.prefixes
}
