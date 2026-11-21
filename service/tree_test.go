package service_test

import (
	"fmt"
	"strings"
	"testing"
	"testing/fstest"

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

func TestTreeFoldersAreCollapsibleDetailsOpenByDefault(t *testing.T) {
	site := build(t, designRepo(), now, service.BuildOptions{})

	tree := between(t, page(t, site, "about/index.html"), `<aside class="pane side">`, `</aside>`)
	assertInOrder(t, "tree", tree,
		`<details class="fold" open><summary>├── <a href="/">posts/</a></summary>`,
		`│   ├── <a href="/posts/picluster/">picluster.md</a>`+"\n",
		`│   └── <a href="/posts/older/">older.md</a>`+"\n</details>",
		`<details class="fold" open><summary>├── <a href="/tags/">tags/</a></summary>`,
		`│   └── <a class="tag" href="/tags/talos/">talos</a>`+"\n</details>",
		`├── <a class="sel" href="/about/" aria-current="page">about.md</a>`+"\n",
		`└── <a href="/feed.xml">feed.xml</a>`+"\n")
	if strings.Count(tree, "<details") != strings.Count(tree, "</details>") {
		t.Errorf("tree has unbalanced <details>:\n%s", tree)
	}
}

// manyPosts is a Content Repository with n Published Posts, p01.md (oldest)
// to p<n>.md (newest), all tagged with one tag each, t01 to t<n>.
func manyPosts(n int) fstest.MapFS {
	files := map[string]string{}
	for i := 1; i <= n; i++ {
		files[fmt.Sprintf("posts/p%02d.md", i)] = fmt.Sprintf(
			"---\ntitle: P%02d\ndate: 2026-01-%02d\ntags: [t%02d]\n---\n", i, i, i)
	}
	return contentRepo(files)
}

func TestAFolderOfMoreThanTwentyShowsTwentyThenAnEllipsisForTheRest(t *testing.T) {
	site := build(t, manyPosts(25), now, service.BuildOptions{})

	tree := between(t, page(t, site, "index.html"), `<aside class="pane side">`, `</aside>`)
	// Newest first: p25 to p06 are shown, p05 to p01 are behind the ellipsis.
	assertInOrder(t, "posts", tree,
		`│   ├── <a href="/posts/p25/">p25.md</a>`+"\n",
		`│   ├── <a href="/posts/p06/">p06.md</a>`+"\n",
		`<details class="more"><summary>│   └── ...</summary>`,
		`│   ├── <a href="/posts/p05/">p05.md</a>`+"\n",
		`│   └── <a href="/posts/p01/">p01.md</a>`+"\n</details></details>",
		`<details class="fold" open><summary>├── <a href="/tags/">tags/</a></summary>`)
	assertInOrder(t, "tags", tree,
		`│   ├── <a class="tag" href="/tags/t20/">t20</a>`+"\n",
		`<details class="more"><summary>│   └── ...</summary>`,
		`│   ├── <a class="tag" href="/tags/t21/">t21</a>`+"\n",
		`│   └── <a class="tag" href="/tags/t25/">t25</a>`+"\n</details></details>")
	if strings.Count(tree, "<details") != strings.Count(tree, "</details>") {
		t.Errorf("tree has unbalanced <details>:\n%s", tree)
	}
}

func TestAFolderOfExactlyTwentyHasNoEllipsis(t *testing.T) {
	site := build(t, manyPosts(20), now, service.BuildOptions{})

	tree := between(t, page(t, site, "index.html"), `<aside class="pane side">`, `</aside>`)
	if strings.Contains(tree, `class="more"`) {
		t.Errorf("tree of 20 posts and 20 tags has an ellipsis:\n%s", tree)
	}
	assertContains(t, "posts", tree, `│   └── <a href="/posts/p01/">p01.md</a>`+"\n</details>")
}

func TestTheEllipsisIsOpenOnAPageItHides(t *testing.T) {
	site := build(t, manyPosts(25), now, service.BuildOptions{})

	hidden := between(t, page(t, site, "posts/p03/index.html"), `<aside class="pane side">`, `</aside>`)
	assertInOrder(t, "tree on a hidden Post", hidden,
		`<details class="more" open><summary>│   └── ...</summary>`,
		`│   ├── <a class="sel" href="/posts/p03/" aria-current="page">p03.md</a>`)
	// The tags folder's ellipsis stays closed: p03's page isn't in it.
	assertContains(t, "tags on a hidden Post", hidden,
		`<a class="tag" href="/tags/t20/">t20</a>`+"\n"+`<details class="more"><summary>`)

	shown := between(t, page(t, site, "posts/p25/index.html"), `<aside class="pane side">`, `</aside>`)
	if strings.Contains(shown, `<details class="more" open>`) {
		t.Errorf("tree on a shown Post has an open ellipsis:\n%s", shown)
	}
}
