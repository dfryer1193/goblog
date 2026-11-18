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

// testSettings is the site.yaml contentRepo adds when a test doesn't give one.
const testSettings = "title: Test blog\nurl: https://blog.example.com\n"

// contentRepo builds an in-memory Content Repository from path → file text.
// It has a minimal site.yaml unless files has one; delete it from the result
// to test a Content Repository without one.
func contentRepo(files map[string]string) fstest.MapFS {
	repo := fstest.MapFS{"site.yaml": &fstest.MapFile{Data: []byte(testSettings)}}
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

func assertNoFile(t *testing.T, site *models.Site, name string) {
	t.Helper()
	if _, ok := site.Files[name]; ok {
		t.Errorf("output has %s, but shouldn't", name)
	}
}

func TestPostsWithNoOrAFuturePublishDateAreDrafts(t *testing.T) {
	repo := contentRepo(map[string]string{
		"posts/today.md":    "---\ntitle: Today\ndate: 2026-03-10\n---\n",
		"posts/tomorrow.md": "---\ntitle: Tomorrow\ndate: 2026-03-11\n---\n",
		"posts/undated.md":  "---\ntitle: Undated\n---\n",
	})

	site := build(t, repo, now, service.BuildOptions{})

	assertNoProblems(t, site)
	index := page(t, site, "index.html")
	assertContains(t, "index", index, ">Today<")
	for _, draft := range []string{"tomorrow", "undated"} {
		if strings.Contains(index, "/posts/"+draft+"/") {
			t.Errorf("index links to the Draft %s", draft)
		}
		assertNoFile(t, site, "posts/"+draft+"/index.html")
	}
}

func TestPublishDateArrivesAtMidnightInTheBuildTimezone(t *testing.T) {
	newYork, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatalf("loading timezone: %v", err)
	}
	repo := contentRepo(map[string]string{
		"posts/ninth.md": "---\ntitle: Ninth\ndate: 2026-03-09\n---\n",
		"posts/tenth.md": "---\ntitle: Tenth\ndate: 2026-03-10\n---\n",
	})
	// 2026-03-10 in UTC, but still 23:59 on 2026-03-09 in New York.
	justBeforeMidnightInNewYork := time.Date(2026, time.March, 10, 3, 59, 0, 0, time.UTC)
	midnightInNewYork := justBeforeMidnightInNewYork.Add(time.Minute)

	tests := []struct {
		name          string
		at            time.Time
		timezone      *time.Location
		wantPublished bool
	}{
		{"UTC defaults on, already the tenth", justBeforeMidnightInNewYork, nil, true},
		{"New York, still the ninth", justBeforeMidnightInNewYork, newYork, false},
		{"New York, now the tenth", midnightInNewYork, newYork, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			site := build(t, repo, tt.at, service.BuildOptions{Timezone: tt.timezone})

			assertNoProblems(t, site)
			assertContains(t, "index", page(t, site, "index.html"), ">Ninth<")
			_, published := site.Files["posts/tenth/index.html"]
			if published != tt.wantPublished {
				t.Errorf("posts/tenth published = %v, want %v", published, tt.wantPublished)
			}
		})
	}
}

func TestIncludingDraftsRendersThemMarkedAsDrafts(t *testing.T) {
	repo := contentRepo(map[string]string{
		"posts/published.md": "---\ntitle: Published\ndate: 2026-03-01\n---\n",
		"posts/scheduled.md": "---\ntitle: Scheduled\ndate: 2026-04-01\n---\n",
		"posts/b-undated.md": "---\ntitle: B undated\n---\nWork in progress.\n",
		"posts/a-undated.md": "---\ntitle: A undated\n---\nWork in progress.\n",
	})

	site := build(t, repo, now, service.BuildOptions{IncludeDrafts: true})

	assertNoProblems(t, site)
	// Undated Drafts have no Publish Date to sort by, so they come first.
	assertInOrder(t, "index", page(t, site, "index.html"),
		"draft", ">A undated<", "draft", ">B undated<", "2026.04.01", "draft", ">Scheduled<", ">Published<")
	assertContains(t, "undated Draft", page(t, site, "posts/a-undated/index.html"), "draft")
	assertContains(t, "scheduled Draft", page(t, site, "posts/scheduled/index.html"), "2026.04.01")
	if published := page(t, site, "posts/published/index.html"); strings.Contains(published, "draft") {
		t.Errorf("the Published Post is marked as a draft:\n%s", published)
	}
}

func TestInvalidFrontMatterSkipsThatPostAndTheRestStillRenders(t *testing.T) {
	tests := []struct {
		name    string
		post    string
		wantErr string
	}{
		{"missing title", "---\ndate: 2026-01-01\n---\nBody.\n", "title"},
		{"blank title", "---\ntitle: \"  \"\ndate: 2026-01-01\n---\nBody.\n", "title"},
		{"no front matter", "Just Markdown.\n", "front matter"},
		{"unclosed front matter", "---\ntitle: Open\ndate: 2026-01-01\n", "front matter"},
		{"malformed date", "---\ntitle: Bad date\ndate: 2026-1-5\n---\n", "2026-1-5"},
		{"impossible date", "---\ntitle: Bad date\ndate: 2026-02-30\n---\n", "2026-02-30"},
		{"date with a time", "---\ntitle: Bad date\ndate: 2026-01-05T10:00:00Z\n---\n", "2026-01-05T10:00:00Z"},
		{"invalid YAML", "---\ntitle: [unclosed\n---\n", "front matter"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := contentRepo(map[string]string{
				"posts/broken.md": tt.post,
				"posts/fine.md":   "---\ntitle: Fine\ndate: 2026-01-01\n---\n",
			})

			site := build(t, repo, now, service.BuildOptions{})

			if len(site.Problems) != 1 {
				t.Fatalf("got problems %v, want exactly one for posts/broken.md", site.Problems)
			}
			problem := site.Problems[0].String()
			assertContains(t, "problem", problem, "posts/broken.md")
			assertContains(t, "problem", problem, tt.wantErr)
			assertContains(t, "index", page(t, site, "index.html"), ">Fine<")
			page(t, site, "posts/fine/index.html")
			assertNoFile(t, site, "posts/broken/index.html")
		})
	}
}

func TestIndexShowsTagsAndSummaries(t *testing.T) {
	repo := contentRepo(map[string]string{
		"posts/summarised.md": "---\ntitle: Summarised\ndate: 2026-01-02\ntags: [k8s, talos]\n" +
			"summary: The author's own blurb.\n---\n\nFirst paragraph that isn't the summary.\n",
		"posts/fallback.md": "---\ntitle: Fallback\ndate: 2026-01-01\n---\n\n# A heading first\n\n" +
			"The *first* paragraph,\nwrapped over [two lines](https://example.com).\n\nThe second paragraph.\n",
	})

	site := build(t, repo, now, service.BuildOptions{})

	assertNoProblems(t, site)
	index := page(t, site, "index.html")
	assertContains(t, "index", index, "The author&#39;s own blurb.")
	assertContains(t, "index", index, "The first paragraph, wrapped over two lines.")
	assertInOrder(t, "index", index, ">Summarised<", "k8s", "talos", ">Fallback<")
	if strings.Contains(index, "isn&#39;t the summary") || strings.Contains(index, "second paragraph") {
		t.Errorf("index shows more than the summary:\n%s", index)
	}

	post := page(t, site, "posts/summarised/index.html")
	assertInOrder(t, "post", post, "k8s", "talos")
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
