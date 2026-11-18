package service_test

import (
	"path"
	"strings"
	"testing"

	"github.com/dfryer1193/goblog/models"
	"github.com/dfryer1193/goblog/service"
)

func TestRelativeLinksToPostsBecomeThosePostsURLs(t *testing.T) {
	repo := contentRepo(map[string]string{
		"posts/linking.md": "---\ntitle: Linking\ndate: 2026-01-02\n---\n\n" +
			"See [the cluster](./picluster.md), [its setup](../posts/picluster.md#setup) " +
			"and [plain](picluster.md).\n",
		"posts/picluster.md": "---\ntitle: Pi cluster\ndate: 2026-01-01\n---\n\n## Setup\n",
	})

	site := build(t, repo, now, service.BuildOptions{})

	assertNoProblems(t, site)
	post := page(t, site, "posts/linking/index.html")
	assertContains(t, "post", post, `<a href="/posts/picluster/">the cluster</a>`)
	assertContains(t, "post", post, `<a href="/posts/picluster/#setup">its setup</a>`)
	assertContains(t, "post", post, `<a href="/posts/picluster/">plain</a>`)
}

// assertSkippedFor checks that the build skipped exactly the Post at name,
// with a Problem mentioning each of wants, and still rendered posts/fine.md.
func assertSkippedFor(t *testing.T, site *models.Site, name string, wants ...string) {
	t.Helper()
	if len(site.Problems) != 1 {
		t.Fatalf("got problems %v, want exactly one for %s", site.Problems, name)
	}
	problem := site.Problems[0].String()
	assertContains(t, "problem", problem, name+": ")
	for _, want := range wants {
		assertContains(t, "problem", problem, want)
	}
	assertNoFile(t, site, "posts/"+strings.TrimSuffix(path.Base(name), ".md")+"/index.html")
	page(t, site, "posts/fine/index.html")
}

func TestBrokenReferencesSkipThatPostAndTheRestStillRenders(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{"link to a missing Post", "[gone](./gone.md)", `"./gone.md"`},
		{"link to a missing Post with a fragment", "[gone](../posts/gone.md#top)", `"../posts/gone.md#top"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := contentRepo(map[string]string{
				"posts/broken.md": "---\ntitle: Broken\ndate: 2026-01-02\n---\n\nSee " + tt.body + ".\n",
				"posts/fine.md":   "---\ntitle: Fine\ndate: 2026-01-01\n---\n",
			})

			site := build(t, repo, now, service.BuildOptions{})

			assertSkippedFor(t, site, "posts/broken.md", tt.want, "not found")
		})
	}
}
