package stats

import "time"

// streamMarker is the value of every line's stream field, which tells Site
// Statistics lines apart from goblog's ordinary logs on stdout.
const streamMarker = "goblog_stats"

// The classes of request a line can be.
const (
	// classPageView is a Page View.
	classPageView = "page_view"
	// classBot is a Bot's request.
	classBot = "bot"
	// classFeedFetch is a known feed reader's request for the feed.
	classFeedFetch = "feed_fetch"
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
	// Bot is a Bot request's Bot name, or botUnknown.
	Bot string `json:"bot,omitempty"`
	// Reader is a feed fetch's feed reader.
	Reader string `json:"reader,omitempty"`
	// Subscribers is the subscriber count a feed fetch's feed reader
	// reports, if it reports one.
	Subscribers *int `json:"subscribers,omitempty"`
}
