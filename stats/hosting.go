package stats

import (
	_ "embed"
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// hostingList is the vendored hosting-network list: AS numbers, one per
// line, ascending. See networks/ORIGIN.md.
//
//go:embed networks/hosting-asns.txt
var hostingList string

// hostingNetworks are the AS numbers of hosting and datacenter networks,
// ascending. A sorted slice is a few tens of KiB, a fraction of a map's.
type hostingNetworks []uint32

// parsedHostingNetworks parses the vendored hosting-network list.
func parsedHostingNetworks() (hostingNetworks, error) {
	h := make(hostingNetworks, 0, strings.Count(hostingList, "\n"))
	for l := range strings.Lines(hostingList) {
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}
		n, err := strconv.ParseUint(l, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("parsing AS number %q: %w", l, err)
		}
		h = append(h, uint32(n))
	}
	slices.Sort(h)
	return h, nil
}

// contains reports whether asn is a hosting network's.
func (h hostingNetworks) contains(asn uint32) bool {
	_, found := slices.BinarySearch(h, asn)
	return found
}
