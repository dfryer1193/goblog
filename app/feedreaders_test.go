package app_test

import (
	"net/http"
	"testing"
)

func TestFeedReadersFetchingTheFeedEmitAFeedFetchLine(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, userAgent, reader string
		// subscribers is the count the line reports, or nil for none.
		subscribers any
	}{
		{
			"Inoreader, documented",
			"Inoreader/1.0 (+http://www.inoreader.com/feed-fetcher; 3 subscribers; )",
			"Inoreader", float64(3),
		},
		{
			"Inoreader, other form",
			"Mozilla/5.0 (compatible; inoreader.com; 114 subscribers)",
			"Inoreader", float64(114),
		},
		{"Feedbin", "Feedbin feed-id:1234 - 21 subscribers", "Feedbin", float64(21)},
		{"Feedbin, one", "Feedbin feed-id:1234 - 1 subscribers", "Feedbin", float64(1)},
		{
			"NewsBlur, one",
			"NewsBlur Feed Fetcher - 1 subscriber - https://www.newsblur.com/site/1234/blog " +
				`("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Safari/605.1.15")`,
			"NewsBlur", float64(1),
		},
		{
			"NewsBlur, several",
			"NewsBlur Feed Fetcher - 12 subscribers - https://www.newsblur.com/site/1234/blog " +
				`("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Safari/605.1.15")`,
			"NewsBlur", float64(12),
		},
		{"Feedly", "Feedly/1.0 (+http://www.feedly.com/fetcher.html; 462 subscribers; )", "Feedly", float64(462)},
		{
			"The Old Reader",
			"Mozilla/5.0 (compatible; theoldreader.com; 1 subscribers; feed-id=cead6c3a837650eeefd8fe4a)",
			"The Old Reader", float64(1),
		},
		{
			"BazQux",
			"Mozilla/5.0 (compatible; BazQux/2.4; +https://bazqux.com/fetcher; 1 subscribers)",
			"BazQux", float64(1),
		},
		{
			"Google Feedfetcher",
			"Feedfetcher-Google; (+http://www.google.com/feedfetcher.html; 7 subscribers; feed-id=728742641706423)",
			"Google Feedfetcher", float64(7),
		},
		{"Feedly, no count", "Feedly/1.0 (+http://www.feedly.com/fetcher.html; like FeedFetcher-Google)", "Feedly", nil},
		{"Miniflux", "Mozilla/5.0 (compatible; Miniflux/2.3.5; +https://miniflux.app)", "Miniflux", nil},
		{"FreshRSS", "FreshRSS/1.31.0 (Linux; https://freshrss.org) like Googlebot", "FreshRSS", nil},
		{"CommaFeed", "CommaFeed/7.5.3 (https://github.com/Athou/commafeed)", "CommaFeed", nil},
		{"NetNewsWire", "NetNewsWire (RSS Reader; https://netnewswire.com/)", "NetNewsWire", nil},
		{
			"NetNewsWire, extended",
			"NetNewsWire (RSS Reader; https://netnewswire.com/; macOS; 6.1.8 (6136))",
			"NetNewsWire", nil,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			g := startGoblog(t, statsSite(t), nil)
			g.sync(t)

			resp := g.request(t, http.MethodGet, "/feed.xml", http.Header{"User-Agent": {tc.userAgent}})
			assertStatus(t, "/feed.xml", resp, http.StatusOK)

			lines := g.statsLines(t, 1)
			assertLine(t, lines[0], map[string]any{
				"class":       "feed_fetch",
				"reader":      tc.reader,
				"subscribers": tc.subscribers,
				"path":        "/feed.xml",
				"status":      float64(http.StatusOK),
				"kind":        "feed",
				"bot":         nil,
			})
		})
	}
}

func TestAFeedReaderRequestingAnythingButTheFeedIsABot(t *testing.T) {
	t.Parallel()
	g := startGoblog(t, statsSite(t), nil)
	g.sync(t)

	g.request(t, http.MethodGet, "/posts/hello/", http.Header{
		"User-Agent": {"Feedly/1.0 (+http://www.feedly.com/fetcher.html; 12 subscribers; )"},
	})

	lines := g.statsLines(t, 1)
	assertLine(t, lines[0], map[string]any{"class": "bot", "bot": "Feedly", "path": "/posts/hello/", "reader": nil})
}

func TestABrowserOrBotFetchingTheFeedIsNoFeedFetch(t *testing.T) {
	t.Parallel()
	g := startGoblog(t, statsSite(t), nil)
	g.sync(t)

	g.get(t, "/feed.xml")
	g.request(t, http.MethodGet, "/feed.xml", http.Header{
		"User-Agent": {"Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)"},
	})

	lines := g.statsLines(t, 1)
	assertLine(t, lines[0], map[string]any{"class": "bot", "bot": "Googlebot", "path": "/feed.xml"})
	if got := g.metric(t, `goblog_other_requests_total{kind="feed"}`); got != 1 {
		t.Errorf(`goblog_other_requests_total{kind="feed"} = %v, want 1`, got)
	}
}

func TestMetricsCountFeedFetchesAndHoldEachReadersLatestSubscriberCount(t *testing.T) {
	t.Parallel()
	g := startGoblog(t, statsSite(t), nil)
	g.sync(t)

	for _, userAgent := range []string{
		"Inoreader/1.0 (+http://www.inoreader.com/feed-fetcher; 3 subscribers; )",
		"Feedbin feed-id:1234 - 21 subscribers",
		"Inoreader/1.0 (+http://www.inoreader.com/feed-fetcher; 5 subscribers; )",
		"NetNewsWire (RSS Reader; https://netnewswire.com/)",
		"NetNewsWire (RSS Reader; https://netnewswire.com/)",
	} {
		g.request(t, http.MethodGet, "/feed.xml", http.Header{"User-Agent": {userAgent}})
	}
	g.statsLines(t, 5)

	for sample, want := range map[string]float64{
		`goblog_feed_fetches_total{reader="Inoreader"}`:   2,
		`goblog_feed_fetches_total{reader="Feedbin"}`:     1,
		`goblog_feed_fetches_total{reader="NetNewsWire"}`: 2,
		`goblog_feed_subscribers{reader="Inoreader"}`:     5,
		`goblog_feed_subscribers{reader="Feedbin"}`:       21,
		`goblog_other_requests_total{kind="feed"}`:        0,
	} {
		if got := g.metric(t, sample); got != want {
			t.Errorf("%s = %v, want %v", sample, got, want)
		}
	}
}
