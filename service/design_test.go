package service_test

import (
	"regexp"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/dfryer1193/goblog/models"
	"github.com/dfryer1193/goblog/service"
)

// stylesheetLink matches the layout's stylesheet link, capturing its path.
var stylesheetLink = regexp.MustCompile(`<link rel="stylesheet" href="/(assets/[^"]+\.css)">`)

// stylesheet returns the path and text of the stylesheet the page links to.
func stylesheet(t *testing.T, site *models.Site, html string) (string, string) {
	t.Helper()
	m := stylesheetLink.FindStringSubmatch(html)
	if m == nil {
		t.Fatalf("page has no stylesheet link:\n%s", html)
	}
	return m[1], page(t, site, m[1])
}

func TestStylesheetAndFontsAreSelfHostedWithVersionedNames(t *testing.T) {
	site := build(t, contentRepo(nil), now, service.BuildOptions{})

	cssPath, css := stylesheet(t, site, page(t, site, "index.html"))
	if !regexp.MustCompile(`^assets/style\.[0-9a-f]{8,}\.css$`).MatchString(cssPath) {
		t.Errorf("stylesheet %s has no content hash in its name", cssPath)
	}
	fonts := regexp.MustCompile(`url\("/(fonts/JetBrainsMono-(\w+)\.[0-9a-f]{8,}\.woff2)"\)`).
		FindAllStringSubmatch(css, -1)
	var styles []string
	for _, f := range fonts {
		styles = append(styles, f[2])
		if !strings.HasPrefix(page(t, site, f[1]), "wOF2") {
			t.Errorf("%s isn't a woff2 font", f[1])
		}
	}
	if got := strings.Join(styles, " "); got != "Regular Bold Italic" {
		t.Errorf("stylesheet's fonts are %q, want Regular Bold Italic", got)
	}
	assertContains(t, "stylesheet", css, `font-family: "JetBrains Mono"`)

	licence := regexp.MustCompile(`/(licenses/JetBrainsMono-OFL\.txt)`).FindStringSubmatch(css)
	if licence == nil {
		t.Fatalf("stylesheet doesn't point at the font licence:\n%s", css)
	}
	assertContains(t, "licence", page(t, site, licence[1]), "SIL Open Font License")
}

func TestOutputMakesNoThirdPartyRequests(t *testing.T) {
	site := build(t, designRepo(), now, service.BuildOptions{})

	external := regexp.MustCompile(`(?i)(src|href)="(https?:)?//|url\(["']?(https?:)?//|@import`)
	for _, name := range site.Paths() {
		if !strings.HasSuffix(name, ".html") && !strings.HasSuffix(name, ".css") {
			continue
		}
		text := page(t, site, name)
		// Links the author wrote are fine; nothing goblog adds may load from elsewhere.
		text = strings.ReplaceAll(text, `href="https://example.com/"`, "")
		if m := external.FindString(text); m != "" {
			t.Errorf("%s loads %q from another site", name, m)
		}
	}
}

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

func TestTreeListsPostsTagsPagesAndTheFeed(t *testing.T) {
	site := build(t, designRepo(), now, service.BuildOptions{})

	tree := between(t, page(t, site, "tags/index.html"), `<aside class="pane side">`, `</aside>`)
	assertInOrder(t, "tree", tree,
		"~/",
		`├── <a href="/">posts/</a>`,
		`│   ├── <a href="/posts/picluster/">picluster.md</a>`,
		`│   └── <a href="/posts/older/">older.md</a>`,
		`├── <a class="sel" href="/tags/" aria-current="page">tags/</a>`,
		`│   ├── <a class="tag" href="/tags/k8s/">k8s</a>`,
		`│   └── <a class="tag" href="/tags/talos/">talos</a>`,
		`├── <a href="/about/">about.md</a>`,
		`└── <a href="/feed.xml">feed.xml</a>`)

	post := between(t, page(t, site, "posts/older/index.html"), `<aside class="pane side">`, `</aside>`)
	assertContains(t, "tree on a Post", post,
		`└── <a class="sel" href="/posts/older/" aria-current="page">older.md</a>`)
}

// between returns the part of text from the first start to the end that
// follows it, failing the test if either is missing.
func between(t *testing.T, text, start, end string) string {
	t.Helper()
	_, rest, ok := strings.Cut(text, start)
	if !ok {
		t.Fatalf("no %q in:\n%s", start, text)
	}
	inner, _, ok := strings.Cut(rest, end)
	if !ok {
		t.Fatalf("no %q after %q in:\n%s", end, start, text)
	}
	return inner
}

// Status bar windows as the layout writes them.
const (
	sep        = `<span class="sep">//</span>`
	postsWin   = `<a class="win" href="/">0 posts</a>` + sep
	tagsWin    = `<a class="win" href="/tags/">1 tags</a>` + sep
	aboutWin   = `<a class="win" href="/about/">2 about</a>` + sep
	windowsTag = `<nav class="wins" aria-label="site">`
)

// currentWin is the status bar window of the page being shown: " #I " then
// " #W:#T ".
func currentWin(index, link, nameAndTitle string) string {
	return `<a class="win cur" href="` + link + `" aria-current="page"><span class="wi">` + index +
		`</span><span class="wn">` + nameAndTitle + `</span></a>` + sep
}

func TestStatusBarWindowsAreTheNavigationWithThePagesWindowCurrent(t *testing.T) {
	site := build(t, designRepo(), now, service.BuildOptions{})

	tests := []struct{ file, want string }{
		{"index.html", currentWin("0", "/", "posts:index") + tagsWin + aboutWin},
		{"posts/picluster/index.html", currentWin("0", "/", "posts:picluster.md") + tagsWin + aboutWin},
		{"tags/index.html", postsWin + currentWin("1", "/tags/", "tags:all") + aboutWin},
		{"tags/k8s/index.html", postsWin + currentWin("1", "/tags/", "tags:k8s") + aboutWin},
		{"about/index.html", postsWin + tagsWin + currentWin("2", "/about/", "about:about.md")},
		{"404.html", postsWin + tagsWin + aboutWin},
	}
	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			if got := between(t, page(t, site, tt.file), windowsTag, "</nav>"); got != tt.want {
				t.Errorf("windows are\n%s\nwant\n%s", got, tt.want)
			}
		})
	}
}

func TestStatusBarShowsTheSessionThenWindowsThenBuildDateTimeAndHost(t *testing.T) {
	site := build(t, designRepo(), now, service.BuildOptions{})

	bar := between(t, page(t, site, "index.html"), `<header class="tbar">`, "</header>")
	// 15:00 UTC is 11:00 in site.yaml's America/New_York; the host is the
	// first label of the site's URL.
	assertInOrder(t, "status bar", bar,
		`<details class="ham"><summary aria-label="files">☰</summary>`, "</details>",
		`[Werewolf Notes]`, sep, windowsTag, "</nav>",
		`<span class="right">`+sep+"2026.03.10"+sep+"11:00"+sep+"blog"+sep+"</span>")
}

func TestStatusBarHostIsGoblogWhenTheSiteHasNoURL(t *testing.T) {
	site := build(t, contentRepo(nil), now, service.BuildOptions{})

	assertContains(t, "status bar", page(t, site, "index.html"),
		`<span class="right">`+sep+"2026.03.10"+sep+"15:00"+sep+"goblog"+sep+"</span>")
}

func TestEveryFileUnderAssetsAndFontsHasAContentHashInItsName(t *testing.T) {
	site := build(t, designRepo(), now, service.BuildOptions{})

	hashed := regexp.MustCompile(`^(assets|fonts)/[^/]+\.[0-9a-f]{10}\.[a-z0-9]+$`)
	n := 0
	for _, name := range site.Paths() {
		if strings.HasPrefix(name, "assets/") || strings.HasPrefix(name, "fonts/") {
			n++
			if !hashed.MatchString(name) {
				t.Errorf("%s is cached for good but has no content hash in its name", name)
			}
		}
	}
	if n != 4 {
		t.Errorf("got %d files under assets/ and fonts/, want the stylesheet and three fonts", n)
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
	for _, rule := range []string{".pane.side { display: none; }", ".ham { display: block; }", ".tbar .right { display: none; }"} {
		assertContains(t, "narrow-screen rules", narrow, rule)
	}
}

func TestOutputHasNoJavaScript(t *testing.T) {
	repo := designRepo()
	repo["posts/sneaky.md"] = &fstest.MapFile{Data: []byte("---\ntitle: Sneaky\ndate: 2026-01-01\n---\n" +
		"<script>alert(1)</script>\n\n<img src=x onerror=\"alert(1)\">\n\n[click](javascript:alert(1))\n")}
	site := build(t, repo, now, service.BuildOptions{IncludeDrafts: true})

	scripts := regexp.MustCompile(`(?i)<script|\son[a-z]+\s*=|javascript:`)
	for _, name := range site.Paths() {
		if strings.HasSuffix(name, ".js") || strings.HasSuffix(name, ".mjs") {
			t.Errorf("output has the script %s", name)
		}
		if strings.HasSuffix(name, ".html") || strings.HasSuffix(name, ".css") {
			if m := scripts.FindString(page(t, site, name)); m != "" {
				t.Errorf("%s has JavaScript: %q", name, m)
			}
		}
	}
}

// stylesheetPath is the output path of the stylesheet the index links to.
func stylesheetPath(t *testing.T, site *models.Site) string {
	t.Helper()
	name, _ := stylesheet(t, site, page(t, site, "index.html"))
	return name
}

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
	assertContains(t, "index", info, "<span>rendered from goblog</span>")
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

func TestCodeBlocksAreHighlightedWithClassesMappedOntoThePalette(t *testing.T) {
	site := build(t, contentRepo(map[string]string{
		"posts/code.md": "---\ntitle: Code\ndate: 2026-01-05\n---\n" +
			"See [the other one](other.md) and ![a rack](../images/rack.png).\n\n" +
			"```go\n// say hi\nfunc main() { fmt.Println(\"hi\") }\n```\n\n" +
			"```\nno language\n```\n",
		"posts/other.md":    "---\ntitle: Other\ndate: 2026-01-01\n---\nOther.\n",
		"images/rack.png":   "png",
		"pages/colophon.md": "---\ntitle: Colophon\n---\n```yaml\nkey: value\n```\n",
	}), now, service.BuildOptions{})

	assertNoProblems(t, site)
	post := page(t, site, "posts/code/index.html")
	assertInOrder(t, "post", post, `<pre class="chroma">`, `<span class="c1">// say hi`,
		`<span class="kd">func</span>`, `<span class="s">&#34;hi&#34;</span>`, "</pre>")
	assertContains(t, "post", post, "no language")
	// Highlighting doesn't get in the way of rewriting references.
	assertContains(t, "post", post, `<a href="/posts/other/">the other one</a>`)
	assertContains(t, "post", post, `<img src="/images/rack.png" alt="a rack">`)
	assertContains(t, "Page", page(t, site, "colophon/index.html"), `<span class="nt">key</span>`)
	for _, name := range site.Paths() {
		if strings.HasSuffix(name, ".html") && strings.Contains(page(t, site, name), "style=") {
			t.Errorf("%s has inline styles", name)
		}
	}

	css := page(t, site, stylesheetPath(t, site))
	for _, rule := range []string{
		".chroma .kd { color: #b7416e }",                     // keywords: magenta
		".chroma .s { color: #bde077 }",                      // strings: bright green
		".chroma .c1 { color: #666666; font-style: italic }", // comments: dim
		".chroma .nt { color: #66aabb }",                     // tags and keys: blue
	} {
		assertContains(t, "stylesheet", css, rule)
	}
}
