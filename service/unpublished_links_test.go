package service_test

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/dfryer1193/goblog/models"
	"github.com/dfryer1193/goblog/service"
)

// unpublishedRepo has a Published Post and a Page that link to a Draft, a
// scheduled Post, a Post skipped for a Problem, a Page skipped for a Problem
// and a Published Post.
func unpublishedRepo() map[string]string {
	return map[string]string{
		"posts/linking.md": "---\ntitle: Linking\ndate: 2026-03-01\n---\n" +
			"See [the **draft**](./draft.md), [soon](../posts/scheduled.md#part), [broken](broken.md), " +
			"[gone page](../pages/untitled.md) and [fine](./fine.md).\n",
		"pages/about.md": "---\ntitle: About\n---\n" +
			"See [the **draft**](../posts/draft.md), [soon](../posts/scheduled.md), [broken](../posts/broken.md), " +
			"[gone page](untitled.md) and [fine](../posts/fine.md).\n",
		"posts/draft.md":     "---\ntitle: Draft\n---\nWIP.\n",
		"posts/scheduled.md": "---\ntitle: Scheduled\ndate: 2026-04-01\n---\nSoon.\n",
		"posts/broken.md":    "---\ndate: 2026-01-01\n---\nNo title.\n",
		"posts/fine.md":      "---\ntitle: Fine\ndate: 2026-01-01\n---\nFine.\n",
		"pages/untitled.md":  "---\n---\nNo title.\n",
	}
}

// warningStrings formats the site's warnings.
func warningStrings(site *models.Site) []string {
	var ws []string
	for _, w := range site.Warnings {
		ws = append(ws, w.String())
	}
	return ws
}

func TestLinksToPostsAndPagesThatArentPublishedArePlainTextWithAWarning(t *testing.T) {
	site := build(t, contentRepo(unpublishedRepo()), now, service.BuildOptions{})

	// The skipped files are still Problems; the links to them aren't.
	if len(site.Problems) != 2 {
		t.Fatalf("got problems %v, want one each for posts/broken.md and pages/untitled.md", site.Problems)
	}
	for _, name := range []string{"posts/linking/index.html", "about/index.html"} {
		body := page(t, site, name)
		assertContains(t, name, body, "See the <strong>draft</strong>, soon, broken, gone page and ")
		assertContains(t, name, body, `<a href="/posts/fine/">fine</a>`)
		for _, target := range []string{"/posts/draft/", "/posts/scheduled/", "/posts/broken/", "/untitled/"} {
			if strings.Contains(body, `href="`+target) {
				t.Errorf("%s links to %s, which isn't published", name, target)
			}
		}
	}

	want := []string{
		`posts/linking.md: link "./draft.md" points at posts/draft.md, which isn't published, so it's plain text`,
		`posts/linking.md: link "../posts/scheduled.md#part" points at posts/scheduled.md, ` +
			`which isn't published, so it's plain text`,
		`posts/linking.md: link "broken.md" points at posts/broken.md, which isn't published, so it's plain text`,
		`posts/linking.md: link "../pages/untitled.md" points at pages/untitled.md, ` +
			`which isn't published, so it's plain text`,
		`pages/about.md: link "../posts/draft.md" points at posts/draft.md, which isn't published, so it's plain text`,
	}
	got := warningStrings(site)
	for _, w := range want {
		if !slices.Contains(got, w) {
			t.Errorf("no warning %q; got %q", w, got)
		}
	}
	if len(got) != 8 {
		t.Errorf("got %d warnings, want 4 each for posts/linking.md and pages/about.md: %q", len(got), got)
	}
}

func TestLinksToAPostRenderOnceItPublishes(t *testing.T) {
	site := build(t, contentRepo(unpublishedRepo()), time.Date(2026, time.April, 1, 12, 0, 0, 0, time.UTC),
		service.BuildOptions{})

	assertContains(t, "post", page(t, site, "posts/linking/index.html"), `<a href="/posts/scheduled/#part">soon</a>`)
	for _, w := range warningStrings(site) {
		if strings.Contains(w, "scheduled.md") {
			t.Errorf("warning about a Post that's now published: %s", w)
		}
	}
}

func TestWithDraftsIncludedLinksToDraftsRender(t *testing.T) {
	site := build(t, contentRepo(unpublishedRepo()), now, service.BuildOptions{IncludeDrafts: true})

	post := page(t, site, "posts/linking/index.html")
	assertContains(t, "post", post, `<a href="/posts/draft/">the <strong>draft</strong></a>`)
	assertContains(t, "post", post, `<a href="/posts/scheduled/#part">soon</a>`)
	assertContains(t, "post", post, "broken, gone page and ")
	for _, w := range warningStrings(site) {
		if strings.Contains(w, "draft.md") || strings.Contains(w, "scheduled.md") {
			t.Errorf("warning about a Draft in a build that includes them: %s", w)
		}
	}
}

func TestADraftsOwnLinksAreOnlyCheckedWhenItIsRendered(t *testing.T) {
	repo := contentRepo(map[string]string{
		"posts/draft.md": "---\ntitle: Draft\n---\nSee [the other](./other.md).\n",
		"posts/other.md": "---\ntitle: Other\n---\nWIP.\n",
	})

	site := build(t, repo, now, service.BuildOptions{})

	assertNoProblems(t, site)
	if len(site.Warnings) != 0 {
		t.Errorf("got warnings %q for a Draft that isn't rendered", warningStrings(site))
	}
}
