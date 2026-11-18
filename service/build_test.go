package service_test

import (
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/dfryer1193/goblog/models"
	"github.com/dfryer1193/goblog/service"
)

// now is the fixed clock most tests build at: mid-afternoon UTC, so the
// date is the same in UTC and America/New_York.
var now = time.Date(2026, time.March, 10, 15, 0, 0, 0, time.UTC) //nolint:gochecknoglobals // test fixture

// contentRepo builds an in-memory Content Repository from path → file text.
func contentRepo(files map[string]string) fstest.MapFS {
	repo := fstest.MapFS{}
	for name, text := range files {
		repo[name] = &fstest.MapFile{Data: []byte(text)}
	}
	return repo
}

// build runs the site build and fails the test if the build as a whole fails.
func build(t *testing.T, repo fstest.MapFS, at time.Time, opts service.BuildOptions) *models.Site {
	t.Helper()
	site, err := service.Build(repo, at, opts)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	return site
}

// page returns an output file's text, failing the test if it doesn't exist.
func page(t *testing.T, site *models.Site, name string) string {
	t.Helper()
	data, ok := site.Files[name]
	if !ok {
		t.Fatalf("output has no %s; it has %v", name, site.Paths())
	}
	return string(data)
}

func assertContains(t *testing.T, name, text, want string) {
	t.Helper()
	if !strings.Contains(text, want) {
		t.Errorf("%s does not contain %q:\n%s", name, want, text)
	}
}

func assertNoProblems(t *testing.T, site *models.Site) {
	t.Helper()
	for _, p := range site.Problems {
		t.Errorf("unexpected problem: %s", p)
	}
}

// assertInOrder checks that each of wants appears in text, in that order.
func assertInOrder(t *testing.T, name, text string, wants ...string) {
	t.Helper()
	rest := text
	for _, want := range wants {
		i := strings.Index(rest, want)
		if i < 0 {
			t.Fatalf("%s does not contain %q after the earlier entries %v:\n%s", name, want, wants, text)
		}
		rest = rest[i+len(want):]
	}
}

func TestIndexListsPostsByPublishDateNewestFirstWithTiesByFilename(t *testing.T) {
	repo := contentRepo(map[string]string{
		"posts/old.md":     "---\ntitle: Old\ndate: 2025-12-31\n---\n",
		"posts/b-same.md":  "---\ntitle: B same day\ndate: 2026-02-01\n---\n",
		"posts/newest.md":  "---\ntitle: Newest\ndate: 2026-03-10\n---\n",
		"posts/a-same.md":  "---\ntitle: A same day\ndate: 2026-02-01\n---\n",
		"posts/middle.md":  "---\ntitle: Middle\ndate: 2026-02-14\n---\n",
		"posts/c-same.md":  "---\ntitle: C same day\ndate: 2026-02-01\n---\n",
		"posts/notes.txt":  "not a Post",
		"drafts/ignore.md": "---\ntitle: Not in posts/\ndate: 2026-01-01\n---\n",
	})

	site := build(t, repo, now, service.BuildOptions{})

	assertNoProblems(t, site)
	assertInOrder(t, "index", page(t, site, "index.html"),
		">Newest<", ">Middle<", ">A same day<", ">B same day<", ">C same day<", ">Old<")
	if strings.Contains(page(t, site, "index.html"), "Not in posts/") {
		t.Error("index lists a Markdown file from outside posts/")
	}
}

func TestPublishedPostAppearsOnIndexAndHasItsOwnPage(t *testing.T) {
	repo := contentRepo(map[string]string{
		"posts/picluster.md": "---\ntitle: Building a Pi cluster\ndate: 2026-01-05\n---\n\n" +
			"Some *Markdown* text.\n\n| a | b |\n|---|---|\n| 1 | ~~2~~ |\n",
	})

	site := build(t, repo, now, service.BuildOptions{})

	assertNoProblems(t, site)
	index := page(t, site, "index.html")
	assertContains(t, "index", index, `<a href="/posts/picluster/">Building a Pi cluster</a>`)
	assertContains(t, "index", index, "2026.01.05")

	post := page(t, site, "posts/picluster/index.html")
	assertContains(t, "post", post, "<h1>Building a Pi cluster</h1>")
	assertContains(t, "post", post, "2026.01.05")
	assertContains(t, "post", post, "<em>Markdown</em>")
	assertContains(t, "post", post, "<table>")
	assertContains(t, "post", post, "<del>2</del>")
}
