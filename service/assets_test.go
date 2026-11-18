package service_test

import (
	"regexp"
	"strings"
	"testing"

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
		// Neither are links to the site itself, such as the feed's autodiscovery link.
		text = strings.ReplaceAll(text, `href="https://blog.example.com/`, "")
		if m := external.FindString(text); m != "" {
			t.Errorf("%s loads %q from another site", name, m)
		}
	}
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

// stylesheetPath is the output path of the stylesheet the index links to.
func stylesheetPath(t *testing.T, site *models.Site) string {
	t.Helper()
	name, _ := stylesheet(t, site, page(t, site, "index.html"))
	return name
}
