package app_test

import (
	"net/http"
	"testing"
)

// assertLine fails the test unless line has each of want's fields, with
// its value, besides the stream marker and the test clock's start time.
func assertLine(t *testing.T, line map[string]any, want map[string]any) {
	t.Helper()
	want["stream"] = "goblog_stats"
	want["time"] = "2026-03-10T15:00:00Z"
	for field, value := range want {
		if line[field] != value {
			t.Errorf("%s = %v, want %v; line: %v", field, line[field], value, line)
		}
	}
}

func TestBotsRequestsEmitABotLineAndNoPageView(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, userAgent, bot string
	}{
		{"Googlebot", "Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)", "Googlebot"},
		{"Bingbot", "Mozilla/5.0 (compatible; bingbot/2.0; +http://www.bing.com/bingbot.htm)", "bingbot"},
		{
			"GPTBot",
			"Mozilla/5.0 AppleWebKit/537.36 (KHTML, like Gecko; compatible; GPTBot/1.2; +https://openai.com/gptbot)",
			"GPTBot",
		},
		{
			"ClaudeBot",
			"Mozilla/5.0 AppleWebKit/537.36 (KHTML, like Gecko; compatible; ClaudeBot/1.0; +claudebot@anthropic.com)",
			"ClaudeBot",
		},
		{"YandexBot", "Mozilla/5.0 (compatible; YandexBot/3.0; +http://yandex.com/bots)", "YandexBot"},
		{"AhrefsBot", "Mozilla/5.0 (compatible; AhrefsBot/7.0; +http://ahrefs.com/robot/)", "AhrefsBot"},
		{"no User-Agent", "", "unknown"},
		{"curl", "curl/8.5.0", "unknown"},
		{"python-requests", "python-requests/2.32.3", "unknown"},
		{"Go-http-client", "Go-http-client/1.1", "unknown"},
		{"Wget", "Wget/1.21.4", "unknown"},
		{"python-urllib3", "python-urllib3/2.2.1", "unknown"},
		{"Java", "Java/17.0.2", "unknown"},
		{"Java-http-client", "Java-http-client/21.0.1", "unknown"},
		{"Guzzle", "GuzzleHttp/7", "unknown"},
		{"Ruby", "Ruby", "unknown"},
		{"Faraday", "Faraday v2.15.1", "unknown"},
		{"Dart", "Dart/3.5 (dart:io)", "unknown"},
		{"Deno", "Deno/2.9.8", "unknown"},
		{"Node fetch", "node", "unknown"},
		{"undici", "undici", "unknown"},
		{"ureq", "ureq/3.0.0", "unknown"},
		{"xh", "xh/0.26.3", "unknown"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			g := startGoblog(t, statsSite(t), nil)
			g.sync(t)

			resp := g.request(t, http.MethodGet, "/posts/hello/", http.Header{"User-Agent": {tc.userAgent}})
			assertStatus(t, "/posts/hello/", resp, http.StatusOK)

			lines := g.statsLines(t, 1)
			assertLine(t, lines[0], map[string]any{
				"class":  "bot",
				"bot":    tc.bot,
				"path":   "/posts/hello/",
				"status": float64(http.StatusOK),
				"kind":   "post",
			})
		})
	}
}

func TestMetricsCountBotRequestsByBotStatusClassAndKindOfPage(t *testing.T) {
	t.Parallel()
	g := startGoblog(t, statsSite(t), nil)
	g.sync(t)
	googlebot := http.Header{"User-Agent": {"Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)"}}
	curl := http.Header{"User-Agent": {"curl/8.5.0"}}

	for _, req := range []struct {
		path   string
		header http.Header
	}{
		{"/posts/hello/", googlebot},
		{"/posts/hello/", googlebot},
		{"/images/cat.png", googlebot},
		{"/posts/hello", googlebot},
		{"/wp-login.php", curl},
		{"/wp-login.php", http.Header{"User-Agent": {""}}},
		{"/feed.xml", curl},
	} {
		g.request(t, http.MethodGet, req.path, req.header)
	}

	for sample, want := range map[string]float64{
		`goblog_bot_requests_total{bot="Googlebot",kind="post",status_class="2xx"}`:  2,
		`goblog_bot_requests_total{bot="Googlebot",kind="image",status_class="2xx"}`: 1,
		`goblog_bot_requests_total{bot="Googlebot",kind="other",status_class="3xx"}`: 1,
		`goblog_bot_requests_total{bot="unknown",kind="missing",status_class="4xx"}`: 2,
		`goblog_bot_requests_total{bot="unknown",kind="feed",status_class="2xx"}`:    1,
		`goblog_page_views_total{kind="post"}`:                                       0,
		`goblog_other_requests_total{kind="image"}`:                                  0,
		`goblog_other_requests_total{kind="missing"}`:                                0,
		`goblog_other_requests_total{kind="feed"}`:                                   0,
	} {
		if got := g.metric(t, sample); got != want {
			t.Errorf("%s = %v, want %v", sample, got, want)
		}
	}
}
