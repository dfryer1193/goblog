package service_test

import (
	"testing"

	"github.com/dfryer1193/goblog/service"
)

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

func TestWithoutAFeedTheTreeEndsWithItsLastEntry(t *testing.T) {
	repo := contentRepo(map[string]string{
		"posts/x.md": "---\ntitle: X\ndate: 2026-01-01\ntags: [k8s, talos]\n---\n",
	})
	delete(repo, "site.yaml")

	site := build(t, repo, now, service.BuildOptions{})

	tree := between(t, page(t, site, "index.html"), `<aside class="pane side">`, `</aside>`)
	assertInOrder(t, "tree", tree,
		`├── <a class="sel" href="/" aria-current="page">posts/</a>`,
		`│   └── <a href="/posts/x/">x.md</a>`,
		`└── <a href="/tags/">tags/</a>`,
		`    ├── <a class="tag" href="/tags/k8s/">k8s</a>`,
		`    └── <a class="tag" href="/tags/talos/">talos</a>`)
}
