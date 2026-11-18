package service_test

import (
	"strings"
	"testing"

	"github.com/dfryer1193/goblog/service"
)

// infoLine returns the page's info line, the plain line along the bottom.
func infoLine(t *testing.T, html string) string {
	t.Helper()
	return between(t, html, `<footer class="info">`, "</footer>")
}

func TestInfoLineSaysWhichCommitWasRenderedAndOnTheIndexThePostCountAndFeed(t *testing.T) {
	commit := "0123456789abcdef0123456789abcdef01234567"
	site := build(t, designRepo(), now, service.BuildOptions{Commit: commit})

	for _, p := range designPages {
		assertContains(t, p.file, infoLine(t, page(t, site, p.file)), "<span>rendered from blog@0123456</span>")
	}
	assertContains(t, "index", infoLine(t, page(t, site, "index.html")),
		`<span>2 posts · <a href="/feed.xml">feed.xml</a></span>`)
	for _, file := range []string{"tags/index.html", "about/index.html", "404.html"} {
		if info := infoLine(
			t,
			page(t, site, file),
		); strings.Contains(info, "posts ·") ||
			strings.Contains(info, "words") {
			t.Errorf("%s's info line has more than the commit: %s", file, info)
		}
	}
}

func TestInfoLineLeavesOutTheCommitWhenItIsUnknown(t *testing.T) {
	site := build(t, contentRepo(map[string]string{
		"posts/only.md": "---\ntitle: Only\ndate: 2026-01-01\n---\nOne.\n",
	}), now, service.BuildOptions{})

	info := infoLine(t, page(t, site, "index.html"))
	assertContains(t, "index", info, "<span>rendered from blog</span>")
	assertContains(t, "index", info, "<span>1 post · ")
}

func TestPostInfoLineHasThePublishDateWordCountAndReadingTime(t *testing.T) {
	tests := []struct{ name, body, want string }{
		{"one word", "Hello.", "published 2026.01.05 · 1 word · ~1 min read"},
		{"markup and code count, by whitespace", "**Bold** and `inline code`, a\n[link](https://example.com/).\n\n" +
			"- item\n\n```sh\necho hi\n```\n", "published 2026.01.05 · 9 words · ~1 min read"},
		// 600 words at 230 a minute is 2.6 minutes.
		{
			"rounds to the nearest minute",
			strings.Repeat("word ", 600),
			"published 2026.01.05 · 600 words · ~3 min read",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			site := build(t, contentRepo(map[string]string{
				"posts/x.md": "---\ntitle: X\ndate: 2026-01-05\n---\n" + tt.body,
			}), now, service.BuildOptions{})

			assertContains(t, "post", infoLine(t, page(t, site, "posts/x/index.html")), "<span>"+tt.want+"</span>")
		})
	}
}

func TestUndatedDraftInfoLineSaysDraft(t *testing.T) {
	site := build(t, contentRepo(map[string]string{
		"posts/wip.md": "---\ntitle: WIP\n---\nNot yet.\n",
	}), now, service.BuildOptions{IncludeDrafts: true})

	assertContains(t, "draft", infoLine(t, page(t, site, "posts/wip/index.html")),
		"<span>draft · 2 words · ~1 min read</span>")
}
