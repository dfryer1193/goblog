package stats

import "time"

// streamMarker is the value of every line's stream field, which tells Site
// Statistics lines apart from goblog's ordinary logs on stdout.
const streamMarker = "goblog_stats"

// The classes of request a line can be.
const (
	// classPageView is a Page View.
	classPageView = "page_view"
)

// line is one Site Statistics JSON line. Its field names are a contract
// (see the README): they're only ever added to.
type line struct {
	// Stream is always streamMarker.
	Stream string `json:"stream"`
	// Time is when the request was answered, in UTC.
	Time time.Time `json:"time"`
	// Class is the request's class, such as classPageView.
	Class string `json:"class"`
	// Path is the request's path, without its query.
	Path string `json:"path"`
	// Status is the response's status code.
	Status int `json:"status"`
	// Kind is the kind of page, such as kindPost.
	Kind string `json:"kind"`
	// Browser is a Page View's browser family, such as "Firefox", or
	// "other".
	Browser string `json:"browser,omitempty"`
	// BrowserMajor is a Page View's browser's major version, such as
	// "145". It's omitted when it isn't known.
	BrowserMajor string `json:"browser_major,omitempty"`
	// OS is a Page View's operating system family, such as "macOS", or
	// "other".
	OS string `json:"os,omitempty"`
	// Device is a Page View's device type, such as "mobile", or "unknown".
	Device string `json:"device,omitempty"`
}
