package stats

import (
	"regexp"
	"strconv"
	"strings"
)

// feedReaders recognises feed readers by their User-Agent: the crawler
// list's feed readers, and a few the list misses.
type feedReaders struct {
	crawlers *crawlers
	// subscribers finds the subscriber count in a feed reader's
	// User-Agent.
	subscribers *regexp.Regexp
}

// feedReader is a feed reader's request.
type feedReader struct {
	// name is the feed reader's name.
	name string
	// subscribers is the subscriber count it reports, if it reports one.
	subscribers *int
}

// newFeedReaders recognises the feed readers in crawlers, and goblog's own.
func newFeedReaders(c *crawlers) *feedReaders {
	return &feedReaders{
		crawlers: c,
		// Inoreader (https://www.inoreader.com/feed-fetcher), Feedbin
		// (https://feedbin.com/help/subscriber-statistics/) and NewsBlur
		// (its source) document "N subscribers", or "1 subscriber". Feedly,
		// The Old Reader, BazQux and Google Feedfetcher are seen sending
		// the same, but don't document it.
		subscribers: regexp.MustCompile(`\b(\d+) subscribers?\b`),
	}
}

// match returns the feed reader with userAgent, if it's a known one.
func (f *feedReaders) match(userAgent string) (feedReader, bool) {
	name, ok := unlistedFeedReader(userAgent)
	if cr, listed := f.crawlers.matchFeedReader(userAgent); listed {
		name, ok = cr.name, true
	}
	if !ok {
		return feedReader{}, false
	}
	reader := feedReader{name: name}
	if m := f.subscribers.FindStringSubmatch(userAgent); m != nil {
		if n, err := strconv.Atoi(m[1]); err == nil {
			reader.subscribers = &n
		}
	}
	return reader, true
}

// unlistedFeedReader returns the name of the feed reader with userAgent, if
// it's one the crawler list misses.
func unlistedFeedReader(userAgent string) (string, bool) {
	// NetNewsWire's, from its Info.plist and RSWeb's UserAgent.swift.
	if strings.HasPrefix(userAgent, "NetNewsWire (RSS Reader;") {
		return "NetNewsWire", true
	}
	return "", false
}
