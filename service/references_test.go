package service_test

import (
	"io/fs"
	"path"
	"strings"
	"testing"
	"testing/fstest"

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

func TestRelativeImagesBecomeImageURLsAndAreCopiedUnchanged(t *testing.T) {
	png := "\x89PNG\r\n\x1a\n\x00binary"
	repo := contentRepo(map[string]string{
		"posts/photos.md": "---\ntitle: Photos\ndate: 2026-01-01\n---\n\n" +
			"![a rack](../images/rack.png) ![a node](./../images/pi/node%201.jpg)\n\n" +
			"[![thumb](../images/rack.png)](../images/rack.png)\n",
		"images/rack.png":      png,
		"images/pi/node 1.jpg": "jpeg bytes",
	})

	site := build(t, repo, now, service.BuildOptions{})

	assertNoProblems(t, site)
	post := page(t, site, "posts/photos/index.html")
	assertContains(t, "post", post, `<img src="/images/rack.png" alt="a rack">`)
	assertContains(t, "post", post, `<img src="/images/pi/node%201.jpg" alt="a node">`)
	assertContains(t, "post", post, `<a href="/images/rack.png"><img src="/images/rack.png" alt="thumb"></a>`)
	if got := page(t, site, "images/rack.png"); got != png {
		t.Errorf("images/rack.png = %q, want it copied unchanged as %q", got, png)
	}
	if got := page(t, site, "images/pi/node 1.jpg"); got != "jpeg bytes" {
		t.Errorf("images/pi/node 1.jpg = %q, want it copied unchanged", got)
	}
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
		name  string
		body  string
		wants []string
	}{
		{"link to a missing Post", "[gone](./gone.md)", []string{`"./gone.md"`, "not found"}},
		{
			"link to a missing Post with a fragment",
			"[gone](../posts/gone.md#top)",
			[]string{`"../posts/gone.md#top"`, "not found"},
		},
		{
			"link out of the Content Repository",
			"[secret](../../secret.md)",
			[]string{`"../../secret.md"`, "escapes the Content Repository"},
		},
		{
			"link out and back in",
			"[fine](../../repo/posts/fine.md)",
			[]string{`"../../repo/posts/fine.md"`, "escapes the Content Repository"},
		},
		{"missing image", "![gone](../images/gone.png)", []string{`"../images/gone.png"`, "not found"}},
		{
			"image out of the Content Repository",
			"![passwd](../../../etc/passwd)",
			[]string{`"../../../etc/passwd"`, "escapes the Content Repository"},
		},
		{"image outside images/", "![post](./fine.md)", []string{`"./fine.md"`, "isn't on the site"}},
		{
			"link to a file that isn't published",
			"[readme](../README.md)",
			[]string{`"../README.md"`, "isn't on the site"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := contentRepo(map[string]string{
				"posts/broken.md": "---\ntitle: Broken\ndate: 2026-01-02\n---\n\nSee " + tt.body + ".\n",
				"posts/fine.md":   "---\ntitle: Fine\ndate: 2026-01-01\n---\n",
				"README.md":       "# My blog\n",
			})

			site := build(t, repo, now, service.BuildOptions{})

			assertSkippedFor(t, site, "posts/broken.md", tt.wants...)
		})
	}
}

func TestEveryBrokenReferenceInAPostIsReported(t *testing.T) {
	repo := contentRepo(map[string]string{
		"posts/broken.md": "---\ntitle: Broken\ndate: 2026-01-02\n---\n\n[a](./a.md) ![b](../images/b.png)\n",
		"posts/fine.md":   "---\ntitle: Fine\ndate: 2026-01-01\n---\n",
	})

	site := build(t, repo, now, service.BuildOptions{})

	assertSkippedFor(t, site, "posts/broken.md", `"./a.md"`, `"../images/b.png"`)
	if problem := site.Problems[0].String(); strings.Contains(problem, "\n") {
		t.Errorf("problem spans several lines: %q", problem)
	}
}

func TestAbsoluteURLsAndFragmentsAreLeftAlone(t *testing.T) {
	repo := contentRepo(map[string]string{
		"posts/out.md": "---\ntitle: Out\ndate: 2026-01-01\n---\n\n" +
			"[web](https://example.com/a.md) [plain](http://example.com/) [mail](mailto:me@example.com) " +
			"[proto](//example.com/x.png) [site](/posts/other/) [here](#below) " +
			"![remote](https://example.com/pic.png) <https://example.com/auto>\n",
	})

	site := build(t, repo, now, service.BuildOptions{})

	assertNoProblems(t, site)
	post := page(t, site, "posts/out/index.html")
	for _, want := range []string{
		`<a href="https://example.com/a.md">web</a>`,
		`<a href="http://example.com/">plain</a>`,
		`<a href="mailto:me@example.com">mail</a>`,
		`<a href="//example.com/x.png">proto</a>`,
		`<a href="/posts/other/">site</a>`,
		`<a href="#below">here</a>`,
		`<img src="https://example.com/pic.png" alt="remote">`,
		`<a href="https://example.com/auto">https://example.com/auto</a>`,
	} {
		assertContains(t, "post", post, want)
	}
}

func TestUnreferencedImagesAreCopiedToo(t *testing.T) {
	repo := contentRepo(map[string]string{
		"images/og/card.png": "card",
		"notes/private.png":  "not an image folder",
	})

	site := build(t, repo, now, service.BuildOptions{})

	assertNoProblems(t, site)
	if got := page(t, site, "images/og/card.png"); got != "card" {
		t.Errorf("images/og/card.png = %q, want it copied unchanged", got)
	}
	assertNoFile(t, site, "notes/private.png")
}

func TestSymlinksCannotReachOutsideTheContentRepository(t *testing.T) {
	symlink := func(target string) *fstest.MapFile {
		return &fstest.MapFile{Mode: fs.ModeSymlink, Data: []byte(target)}
	}
	repo := contentRepo(map[string]string{
		"posts/fine.md":  "---\ntitle: Fine\ndate: 2026-01-01\n---\n",
		"outside/key":    "secret",
		"outside/me.png": "secret picture",
	})
	repo["images/key.png"] = symlink("../outside/key")
	repo["images/linked"] = symlink("../outside")

	tests := []struct {
		name string
		ref  string
	}{
		{"symlinked image", "../images/key.png"},
		{"image in a symlinked folder", "../images/linked/me.png"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo["posts/broken.md"] = &fstest.MapFile{
				Data: []byte("---\ntitle: Broken\ndate: 2026-01-02\n---\n\n![x](" + tt.ref + ")\n"),
			}

			site := build(t, repo, now, service.BuildOptions{})

			assertSkippedFor(t, site, "posts/broken.md", `"`+tt.ref+`"`, "not found")
			for _, name := range site.Paths() {
				if data := string(site.Files[name]); strings.Contains(data, "secret") {
					t.Errorf("output %s has a file from outside images/: %q", name, data)
				}
			}
		})
	}
}

func TestASymlinkedImagesFolderIsNotCopied(t *testing.T) {
	repo := contentRepo(map[string]string{"outside/me.png": "secret picture"})
	repo["images"] = &fstest.MapFile{Mode: fs.ModeSymlink, Data: []byte("outside")}

	site := build(t, repo, now, service.BuildOptions{})

	assertNoFile(t, site, "images/me.png")
}
