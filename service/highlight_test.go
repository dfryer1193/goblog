package service_test

import (
	"strings"
	"testing"

	"github.com/dfryer1193/goblog/service"
)

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
