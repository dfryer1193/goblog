package service_test

import (
	"regexp"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/dfryer1193/goblog/service"
)

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
