package api

import (
	"strconv"
	"strings"
)

// acceptsGzip reports whether a request with the Accept-Encoding header
// values accepts a gzip-encoded response (RFC 9110, section 12.5.3): gzip
// (or x-gzip), else "*", is listed with a q-value above zero. No header
// means no gzip, since the client may not decode it.
func acceptsGzip(values []string) bool {
	gzipQ, starQ := -1.0, -1.0
	for _, value := range values {
		for item := range strings.SplitSeq(value, ",") {
			coding, q := parseCoding(item)
			switch coding {
			case "gzip", "x-gzip":
				gzipQ = max(gzipQ, q)
			case "*":
				starQ = max(starQ, q)
			}
		}
	}
	if gzipQ >= 0 {
		return gzipQ > 0
	}
	return starQ > 0
}

// parseCoding splits an Accept-Encoding item such as "gzip;q=0.5" into its
// lower-cased coding and q-value. A missing q is 1; a malformed one is 0,
// so the coding isn't used.
func parseCoding(item string) (string, float64) {
	coding, params, _ := strings.Cut(item, ";")
	coding = strings.ToLower(strings.TrimSpace(coding))
	q := 1.0
	for param := range strings.SplitSeq(params, ";") {
		key, value, _ := strings.Cut(param, "=")
		if !strings.EqualFold(strings.TrimSpace(key), "q") {
			continue
		}
		parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
		if err != nil || parsed < 0 || parsed > 1 {
			return coding, 0
		}
		q = parsed
	}
	return coding, q
}
