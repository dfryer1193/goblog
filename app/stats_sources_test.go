package app_test

import (
	"net/http"
	"net/netip"
	"strings"
	"testing"
)

// sourceFields are the fields of a line that say where its Visitor came
// from. A nil value means the field must be absent.
type sourceFields map[string]any

// sourceOf is a line's sourceFields for a Source of kind, named source (""
// for none), that is or isn't an entry, with no Campaign.
func sourceOf(kind, source string, entry bool) sourceFields {
	f := sourceFields{
		"source_kind":     kind,
		"source":          nil,
		"entry":           entry,
		"campaign_source": nil,
		"campaign_medium": nil,
		"campaign_name":   nil,
	}
	if source != "" {
		f["source"] = source
	}
	return f
}

// campaignOf is a line's sourceFields for a Campaign with source, medium
// and name ("" for absent), that is or isn't an entry.
func campaignOf(source, medium, name string, entry bool) sourceFields {
	f := sourceOf("campaign", source, entry)
	f["campaign_source"] = source
	for field, value := range map[string]string{"campaign_medium": medium, "campaign_name": name} {
		if value != "" {
			f[field] = value
		}
	}
	return f
}

// assertSource fails the test unless line has the fields in want.
func assertSource(t *testing.T, line map[string]any, want sourceFields) {
	t.Helper()
	for field, value := range want {
		got, present := line[field]
		switch {
		case value == nil && present:
			t.Errorf("%s = %v, want it absent; line: %v", field, got, line)
		case value != nil && got != value:
			t.Errorf("%s = %v, want %v; line: %v", field, got, value, line)
		}
	}
}

// sourceCase is one request for a Page View, and where its line should say
// the Visitor came from.
type sourceCase struct {
	name string
	// query is the request's query string, without "?".
	query string
	// referer is the Referer header, or "" for none.
	referer string
	want    sourceFields
}

// runSourceCases sends each case's request, for /posts/hello/ on a fresh
// goblog serving repo, and checks its one line.
func runSourceCases(t *testing.T, repo *contentRepo, cases []sourceCase) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			g := startGoblog(t, repo, nil)
			g.sync(t)
			path := "/posts/hello/"
			if tc.query != "" {
				path += "?" + tc.query
			}
			header := http.Header{}
			if tc.referer != "" {
				header.Set("Referer", strings.ReplaceAll(tc.referer, "{goblog}", g.srv.URL))
			}

			assertStatus(t, path, g.request(t, http.MethodGet, path, header), http.StatusOK)

			lines := g.statsLines(t, 1)
			assertPageView(t, lines[0], "/posts/hello/", "post")
			assertSource(t, lines[0], tc.want)
		})
	}
}

func TestAPageViewLineSaysWhichSourceTheVisitorCameFrom(t *testing.T) {
	t.Parallel()
	runSourceCases(t, statsSite(t), []sourceCase{
		// Search engines.
		{"Google", "", "https://www.google.com/", sourceOf("search", "Google", true)},
		{"Google in a country", "", "https://www.google.co.uk/", sourceOf("search", "Google", true)},
		{"Google in another country", "", "https://google.com.au/", sourceOf("search", "Google", true)},
		{"Bing in a country", "", "https://cn.bing.com/", sourceOf("search", "Bing", true)},
		{"DuckDuckGo", "", "https://duckduckgo.com/", sourceOf("search", "DuckDuckGo", true)},
		{"Kagi", "", "https://kagi.com/", sourceOf("search", "Kagi", true)},
		// AI assistants.
		{"an AI assistant", "", "https://chatgpt.com/", sourceOf("ai", "ChatGPT", true)},
		// Social sites.
		{"Twitter's shortener", "", "https://t.co/", sourceOf("social", "Twitter", true)},
		{"Facebook's link redirector", "", "https://l.facebook.com/", sourceOf("social", "Facebook", true)},
		{"Mastodon", "", "https://mastodon.social/", sourceOf("social", "Mastodon", true)},
		{"Bluesky", "", "https://bsky.app/", sourceOf("social", "Bluesky", true)},
		{"Hacker News", "", "https://news.ycombinator.com/", sourceOf("social", "Hacker News", true)},
		{"an app's package name", "", "android-app://com.reddit.frontpage/", sourceOf("social", "reddit", true)},
		// Full referrer URLs are matched by their host, like origins.
		{"a full URL", "", "https://news.ycombinator.com/item?id=1", sourceOf("social", "Hacker News", true)},
		{"an upper-case host", "", "https://News.YCombinator.com/", sourceOf("social", "Hacker News", true)},
		// Other sites, by registrable domain.
		{"a subdomain", "", "https://foo.example.co.uk/", sourceOf("site", "example.co.uk", true)},
		{"a port", "", "https://www.example.com:8443/a/b", sourceOf("site", "example.com", true)},
		{"a site not on the list", "", "https://lobste.rs/", sourceOf("site", "lobste.rs", true)},
		{"a site under a hosting suffix", "", "https://alice.github.io/", sourceOf("site", "alice.github.io", true)},
		{"an IP address", "", "http://192.0.2.1/", sourceOf("site", "ip", true)},
		{
			"an unlisted app",
			"",
			"android-app://com.google.android.gm/",
			sourceOf("site", "com.google.android.gm", true),
		},
		// No Source.
		{"no referrer", "", "", sourceOf("direct", "", true)},
		{"an unreadable referrer", "", "not a url", sourceOf("direct", "", true)},
		{"the site itself", "", "https://blog.example.com/", sourceOf("internal", "", false)},
		{"a page of the site", "", "https://blog.example.com/tags/k8s/", sourceOf("internal", "", false)},
		{"the site in upper case", "", "https://BLOG.example.com/", sourceOf("internal", "", false)},
		{"the site with www", "", "https://www.blog.example.com/", sourceOf("internal", "", false)},
		{"the site on another scheme", "", "http://blog.example.com/", sourceOf("internal", "", false)},
	})
}

func TestACampaignTagIsTheSourceAheadOfTheReferrer(t *testing.T) {
	t.Parallel()
	long := strings.Repeat("a", 200)
	runSourceCases(t, statsSite(t), []sourceCase{
		{"ref", "ref=newsletter", "", campaignOf("newsletter", "", "", true)},
		{"utm_source", "utm_source=mastodon", "", campaignOf("mastodon", "", "", true)},
		{
			"utm_source, utm_medium and utm_campaign", "utm_source=buttondown&utm_medium=email&utm_campaign=launch",
			"", campaignOf("buttondown", "email", "launch", true),
		},
		{"over a referring site", "ref=newsletter", "https://mail.google.com/", campaignOf("newsletter", "", "", true)},
		{
			"over a social site", "utm_source=mastodon&utm_medium=social", "https://mastodon.social/",
			campaignOf("mastodon", "social", "", true),
		},
		{"utm_source over ref", "ref=a&utm_source=b", "", campaignOf("b", "", "", true)},
		{"from the site itself", "ref=footer", "https://blog.example.com/", campaignOf("footer", "", "", false)},
		{
			"normalised", "utm_source=%20Mastodon%20&utm_medium=SOCIAL&utm_campaign=%09Spring%20Launch%0A",
			"", campaignOf("mastodon", "social", "spring launch", true),
		},
		{
			"capped", "utm_source=" + long + "&utm_medium=" + long + "&utm_campaign=" + long,
			"", campaignOf(long[:64], long[:64], long[:64], true),
		},
		{
			"capped by character",
			"ref=" + strings.Repeat("é", 70),
			"",
			campaignOf(strings.Repeat("é", 64), "", "", true),
		},
		// Not Campaigns.
		{"an empty ref", "ref=", "https://lobste.rs/", sourceOf("site", "lobste.rs", true)},
		{"a blank utm_source", "utm_source=%20%20", "", sourceOf("direct", "", true)},
		{"utm_medium alone", "utm_medium=email&utm_campaign=launch", "", sourceOf("direct", "", true)},
		{"another query", "q=hello", "", sourceOf("direct", "", true)},
	})
}

func TestWithoutASiteURLTheHostServingTheSiteIsTheSitesHost(t *testing.T) {
	t.Parallel()
	// With no site.yaml, the site has the default settings, which have no
	// url.
	repo := newContentRepo(t)
	repo.commit(t, map[string]string{"posts/hello.md": post("Hello", "2026-03-01")})
	runSourceCases(t, repo, []sourceCase{
		{"the host serving the site", "", "{goblog}/", sourceOf("internal", "", false)},
		{"another site", "", "https://blog.example.com/", sourceOf("site", "example.com", true)},
	})
}

func TestAReferrerFromAnIPAddressIsTheSourceIPNeverTheAddress(t *testing.T) {
	t.Parallel()
	g := startGoblog(t, statsSite(t), nil)
	g.sync(t)
	referrers := []string{
		"http://192.0.2.1/",
		"https://198.51.100.7:8443/a/b?c=d",
		"http://[2001:db8::1]/",
		"https://[2001:db8::2]:8443/",
		"http://[::ffff:192.0.2.3]/",
		"android-app://203.0.113.4/",
	}
	for _, referer := range referrers {
		g.request(t, http.MethodGet, "/posts/hello/", http.Header{"Referer": {referer}})
	}

	for _, line := range g.statsLines(t, len(referrers)) {
		assertSource(t, line, sourceOf("site", "ip", true))
		if source, ok := line["source"].(string); ok {
			if _, err := netip.ParseAddr(source); err == nil {
				t.Errorf("source = %q, an IP address; line: %v", source, line)
			}
		}
	}
}
