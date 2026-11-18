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
var stylesheetLink = regexp.MustCompile(`<link rel="stylesheet" href="/(static/[^"]+\.css)">`)

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
	if !regexp.MustCompile(`^static/style\.[0-9a-f]{8,}\.css$`).MatchString(cssPath) {
		t.Errorf("stylesheet %s has no content hash in its name", cssPath)
	}
	fonts := regexp.MustCompile(`url\("/(static/JetBrainsMono-(\w+)\.[0-9a-f]{8,}\.woff2)"\)`).
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

	licence := regexp.MustCompile(`/(static/OFL\.[0-9a-f]{8,}\.txt)`).FindStringSubmatch(css)
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
