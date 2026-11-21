package service_test

import (
	"regexp"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/dfryer1193/goblog/service"
)

// treeScript matches the layout's one script: the tree's, from /assets/,
// with a content hash in its name, deferred.
var treeScript = regexp.MustCompile(`<script src="/(assets/tree\.[0-9a-f]{8,}\.js)" defer></script>`)

func TestTheOnlyJavaScriptIsTheTreeScript(t *testing.T) {
	repo := designRepo()
	repo["posts/sneaky.md"] = &fstest.MapFile{Data: []byte("---\ntitle: Sneaky\ndate: 2026-01-01\n---\n" +
		"<script>alert(1)</script>\n\n<img src=x onerror=\"alert(1)\">\n\n[click](javascript:alert(1))\n")}
	site := build(t, repo, now, service.BuildOptions{IncludeDrafts: true})

	script := treeScript.FindStringSubmatch(page(t, site, "index.html"))
	if script == nil {
		t.Fatal("index has no tree script")
	}
	scripts := regexp.MustCompile(`(?i)<script|\son[a-z]+\s*=|javascript:`)
	for _, name := range site.Paths() {
		if (strings.HasSuffix(name, ".js") || strings.HasSuffix(name, ".mjs")) && name != script[1] {
			t.Errorf("output has the script %s", name)
		}
		if strings.HasSuffix(name, ".html") || strings.HasSuffix(name, ".css") {
			text := page(t, site, name)
			if strings.HasSuffix(name, ".html") {
				if n := len(treeScript.FindAllString(text, -1)); n != 1 {
					t.Errorf("%s links the tree script %d times, want once", name, n)
				}
			}
			if m := scripts.FindString(treeScript.ReplaceAllString(text, "")); m != "" {
				t.Errorf("%s has JavaScript: %q", name, m)
			}
		}
	}
	assertContains(t, "tree script", page(t, site, script[1]), "localStorage")
}
