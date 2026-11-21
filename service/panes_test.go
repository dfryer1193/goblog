package service_test

import (
	"strings"
	"testing"
	"testing/fstest"

	"github.com/dfryer1193/goblog/service"
)

// designRepo is a small Content Repository with one of everything the design
// shows: Posts with tags and code, a Page and full site settings.
func designRepo() fstest.MapFS {
	return contentRepo(map[string]string{
		"site.yaml": "title: Werewolf Notes\nurl: https://blog.example.com\ntimezone: America/New_York\n" +
			"nav:\n  - {name: posts, link: /}\n  - {name: tags, link: /tags/}\n  - {name: about, link: /about/}\n",
		"posts/picluster.md": "---\ntitle: Building a Pi cluster\ndate: 2026-01-05\ntags: [k8s, talos]\n---\n" +
			"Four Pis and [a site](https://example.com/).\n\n```go\nfunc main() {}\n```\n",
		"posts/older.md": "---\ntitle: Older\ndate: 2025-12-01\ntags: [k8s]\n---\nOld words.\n",
		"pages/about.md": "---\ntitle: About me\n---\nHi.\n",
	})
}

// designPages are the output pages of designRepo, one of each kind, with the
// title label their active pane should carry.
var designPages = []struct{ kind, file, label string }{ //nolint:gochecknoglobals // test table
	{"index", "index.html", "posts"},
	{"Post", "posts/picluster/index.html", "picluster.md"},
	{"tags", "tags/index.html", "tags"},
	{"tag", "tags/k8s/index.html", "#k8s"},
	{"Page", "about/index.html", "about.md"},
	{"404", "404.html", "[no name]"},
}

func TestEveryPageHasTheTreePaneBesideTheActivePane(t *testing.T) {
	site := build(t, designRepo(), now, service.BuildOptions{})

	assertNoProblems(t, site)
	for _, p := range designPages {
		t.Run(p.kind, func(t *testing.T) {
			html := page(t, site, p.file)
			assertInOrder(t, p.file, html,
				`<aside class="pane side">`, `<span class="label">tree</span>`, `<div class="tree">`, `</aside>`,
				`<main class="pane active`, `<span class="label">`+p.label+`</span>`, `</main>`)
		})
	}
}

func TestNotFoundPageHasARedNoNamePane(t *testing.T) {
	site := build(t, designRepo(), now, service.BuildOptions{})

	assertContains(t, "404", page(t, site, "404.html"),
		`<main class="pane active err"><span class="label">[no name]</span>`)
	for _, p := range designPages[:len(designPages)-1] {
		if strings.Contains(page(t, site, p.file), "pane active err") {
			t.Errorf("%s has the 404's red pane", p.file)
		}
	}
	css := page(t, site, stylesheetPath(t, site))
	assertContains(t, "stylesheet", css, ".pane.active.err { border-color: var(--red); }")
	assertContains(t, "stylesheet", css, ".pane.active.err > .label { color: var(--red); }")
	assertContains(t, "stylesheet", css, "--red: #e84f4f;")
}

func TestOnPhonesADetailsDrawerOpensTheTree(t *testing.T) {
	site := build(t, designRepo(), now, service.BuildOptions{})

	for _, p := range designPages {
		html := page(t, site, p.file)
		drawer := between(t, html, `<details class="ham">`, "</details>")
		assertInOrder(t, p.file, drawer, `<summary aria-label="files">☰</summary>`,
			`<nav class="drawer" aria-label="files"><div class="tree">`, `>picluster.md</a>`)
		assertInOrder(t, p.file, html, `<details class="ham">`, `[Werewolf Notes]`)
	}
	narrow := between(t, page(t, site, stylesheetPath(t, site)), "@media (max-width: 760px) {", "\n}")
	for _, rule := range []string{".pane.side { display: none; }", ".ham { display: block; }"} {
		assertContains(t, "narrow-screen rules", narrow, rule)
	}
}

func TestOnPhonesTheStatusBarScrollsWithMenuAndSessionPinned(t *testing.T) {
	site := build(t, designRepo(), now, service.BuildOptions{})

	narrow := between(t, page(t, site, stylesheetPath(t, site)), "@media (max-width: 760px) {", "\n}")
	for _, rule := range []string{
		".tbar { overflow-x: auto;",
		".ham { position: sticky; left: 0;",
		".tbar .sess { position: sticky; left: var(--ham-w);",
		".tbar::after {",
		"animation-timeline: scroll(nearest inline);",
	} {
		assertContains(t, "narrow-screen rules", narrow, rule)
	}
	if strings.Contains(narrow, ".tbar .right { display: none; }") {
		t.Error("phones still hide the status bar's date, time and host")
	}
}

func TestTheTreeIsSetAtTheFontsNaturalLineHeightSoItsLinesJoin(t *testing.T) {
	site := build(t, designRepo(), now, service.BuildOptions{})

	css := page(t, site, stylesheetPath(t, site))
	assertContains(t, "stylesheet", css, ".tree { white-space: pre; line-height: normal; }")
	narrow := between(t, css, "@media (max-width: 760px) {", "\n}")
	if strings.Contains(narrow, "line-height: 2.1") {
		t.Error("phones still space the drawer's tree out, breaking its lines")
	}
}
