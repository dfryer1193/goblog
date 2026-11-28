package stats

import (
	"net/http"
	"net/url"
	"strings"
	"time"
)

// request is what Site Statistics keep of a request they record: a snapshot
// taken as it's served, which is cheap to take, so that classifying it,
// looking it up and emitting it all happen in the recorder, off the request
// path.
type request struct {
	// at is when the request arrived, in UTC.
	at time.Time
	// excludedNetworks is excluded_networks' value when it arrived.
	excludedNetworks []string

	method string
	// path is the URL's path, without its query.
	path string
	// rawQuery is the URL's query, without "?".
	rawQuery string
	// host is the host the request was sent to.
	host string
	// remoteAddr is the TCP peer's address.
	remoteAddr string

	// The headers that Site Statistics read. forwardedFor is the last
	// X-Forwarded-For header, the gateway's.
	userAgent, referer, forwardedFor, secPurpose, purpose string

	// status is the response's status code.
	status int
	// kind is the kind of page served.
	kind kind
}

// snapshot is the part of r that Site Statistics record, as it arrives at
// at.
func snapshot(r *http.Request, at time.Time) request {
	req := request{
		at:         at,
		method:     r.Method,
		path:       r.URL.Path,
		rawQuery:   r.URL.RawQuery,
		host:       r.Host,
		remoteAddr: r.RemoteAddr,
		userAgent:  r.UserAgent(),
		referer:    r.Header.Get("Referer"),
		secPurpose: r.Header.Get("Sec-Purpose"),
		purpose:    r.Header.Get("Purpose"),
	}
	if values := r.Header.Values("X-Forwarded-For"); len(values) > 0 {
		req.forwardedFor = values[len(values)-1]
	}
	return req
}

// query is the request's parsed query. A malformed query yields what
// parses.
func (req request) query() url.Values {
	values, _ := url.ParseQuery(req.rawQuery)
	return values
}

// isPrefetch reports whether req is a browser's prefetch (or prerender), as
// its Sec-Purpose or Purpose header says.
func (req request) isPrefetch() bool {
	for _, header := range []string{req.secPurpose, req.purpose} {
		if strings.Contains(strings.ToLower(header), "prefetch") {
			return true
		}
	}
	return false
}
