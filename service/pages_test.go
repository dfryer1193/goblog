package service_test

import (
	"strings"
	"testing"

	"github.com/dfryer1193/goblog/service"
)

func TestPageRendersAtItsNameAndStaysOutOfTheIndexAndTags(t *testing.T) {
	repo := contentRepo(map[string]string{
		"pages/about.md": "---\ntitle: About me\ntags: [secret-tag]\n---\n\nI write *things*.\n",
		"posts/x.md":     "---\ntitle: X\ndate: 2026-01-01\n---\n",
	})

	site := build(t, repo, now, service.BuildOptions{})

	assertNoProblems(t, site)
	about := page(t, site, "about/index.html")
	assertContains(t, "page", about, "<title>About me · Test blog</title>")
	assertContains(t, "page", about, "<h1>About me</h1>")
	assertContains(t, "page", about, "<em>things</em>")
	for _, name := range []string{"index.html", "tags/index.html"} {
		if text := page(t, site, name); strings.Contains(text, "About me") || strings.Contains(text, "secret-tag") {
			t.Errorf("%s shows the Page:\n%s", name, text)
		}
	}
	assertNoFile(t, site, "tags/secret-tag/index.html")
}

func TestPagesThatCollideWithTheSitesOwnPathsOrAreInvalidAreProblems(t *testing.T) {
	tests := []struct {
		file    string
		wantErr string
	}{
		{"pages/posts.md", "reserved"},
		{"pages/Tags.md", "reserved"},
		{"pages/images.md", "reserved"},
		{"pages/feed.xml.md", "reserved"},
		{"pages/sitemap.xml.md", "reserved"},
		{"pages/readyz.md", "reserved"},
		{"pages/.md", "name"},
		{"pages/about me.md", "name"},
		{"pages/no-title.md", "title"},
	}
	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			text := "---\ntitle: Clash\n---\nBody.\n"
			if strings.Contains(tt.file, "no-title") {
				text = "---\n---\nBody.\n"
			}
			repo := contentRepo(map[string]string{
				tt.file:          text,
				"pages/about.md": "---\ntitle: About\n---\n",
				"posts/x.md":     "---\ntitle: X\ndate: 2026-01-01\n---\n",
			})

			site := build(t, repo, now, service.BuildOptions{})

			if len(site.Problems) != 1 {
				t.Fatalf("got problems %v, want exactly one for %s", site.Problems, tt.file)
			}
			problem := site.Problems[0].String()
			assertContains(t, "problem", problem, tt.file)
			assertContains(t, "problem", problem, tt.wantErr)
			page(t, site, "about/index.html")
			page(t, site, "posts/x/index.html")
			for _, name := range site.Paths() {
				if strings.Contains(page(t, site, name), "Clash") {
					t.Errorf("%s shows the rejected Page", name)
				}
			}
		})
	}
}

func TestPagesResolveRelativeLinksAndImagesLikePosts(t *testing.T) {
	repo := contentRepo(map[string]string{
		"pages/about.md": "---\ntitle: About\n---\n\n![me](../images/me.png) " +
			"[a post](../posts/x.md#end) [contact](contact.md) [ext](https://example.com/a.md)\n",
		"pages/contact.md": "---\ntitle: Contact\n---\n",
		"images/me.png":    "png",
		"posts/x.md":       "---\ntitle: X\ndate: 2026-01-01\n---\n",
	})

	site := build(t, repo, now, service.BuildOptions{})

	assertNoProblems(t, site)
	about := page(t, site, "about/index.html")
	assertContains(t, "about", about, `src="/images/me.png"`)
	assertContains(t, "about", about, `href="/posts/x/#end"`)
	assertContains(t, "about", about, `href="/contact/"`)
	assertContains(t, "about", about, `href="https://example.com/a.md"`)
	page(t, site, "images/me.png")
}

func TestPageWithBrokenReferencesIsAProblem(t *testing.T) {
	repo := contentRepo(map[string]string{
		"pages/broken.md": "---\ntitle: Broken\n---\n\n[gone](../posts/gone.md) ![away](../../secret.png)\n",
		"pages/about.md":  "---\ntitle: About\n---\n",
	})

	site := build(t, repo, now, service.BuildOptions{})

	if len(site.Problems) != 1 {
		t.Fatalf("got problems %v, want exactly one for pages/broken.md", site.Problems)
	}
	problem := site.Problems[0].String()
	assertContains(t, "problem", problem, "pages/broken.md: ")
	assertContains(t, "problem", problem, `"../posts/gone.md"`)
	assertContains(t, "problem", problem, "not found")
	assertContains(t, "problem", problem, "escapes the Content Repository")
	assertNoFile(t, site, "broken/index.html")
	page(t, site, "about/index.html")
}

func TestPostsLinkToPagesAtTheirURL(t *testing.T) {
	repo := contentRepo(map[string]string{
		"posts/x.md":     "---\ntitle: X\ndate: 2026-01-01\n---\n\n[about](../pages/about.md#me) [gone](../pages/gone.md)\n",
		"posts/fine.md":  "---\ntitle: Fine\ndate: 2026-01-01\n---\n\n[about](../pages/about.md)\n",
		"pages/about.md": "---\ntitle: About\n---\n",
	})

	site := build(t, repo, now, service.BuildOptions{})

	if len(site.Problems) != 1 {
		t.Fatalf("got problems %v, want exactly one for posts/x.md", site.Problems)
	}
	assertContains(
		t,
		"problem",
		site.Problems[0].String(),
		`posts/x.md: link "../pages/gone.md" points at pages/gone.md, which is not found`,
	)
	assertContains(t, "fine", page(t, site, "posts/fine/index.html"), `href="/about/"`)
}
