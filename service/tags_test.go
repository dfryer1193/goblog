package service_test

import (
	"strings"
	"testing"

	"github.com/dfryer1193/goblog/service"
)

func TestTagsPageCountsPublishedPostsAndEachTagListsItsPostsInIndexOrder(t *testing.T) {
	repo := contentRepo(map[string]string{
		"posts/old.md":       "---\ntitle: Old\ndate: 2025-06-01\ntags: [k8s]\n---\n",
		"posts/new.md":       "---\ntitle: New\ndate: 2026-03-01\ntags: [k8s, talos]\n---\n",
		"posts/same-day.md":  "---\ntitle: Same day\ndate: 2026-03-01\ntags: [k8s]\n---\n",
		"posts/untagged.md":  "---\ntitle: Untagged\ndate: 2026-02-01\n---\n",
		"posts/scheduled.md": "---\ntitle: Scheduled\ndate: 2026-04-01\ntags: [k8s, upcoming]\n---\n",
		"posts/undated.md":   "---\ntitle: Undated\ntags: [talos, wip]\n---\n",
	})

	site := build(t, repo, now, service.BuildOptions{})

	assertNoProblems(t, site)
	tags := page(t, site, "tags/index.html")
	assertInOrder(t, "tags", tags,
		`<a href="/tags/k8s/">#k8s</a> (3)`, `<a href="/tags/talos/">#talos</a> (1)`)
	for _, draftOnly := range []string{"upcoming", "wip"} {
		if strings.Contains(tags, draftOnly) {
			t.Errorf("tags lists %s, which only Drafts have:\n%s", draftOnly, tags)
		}
		assertNoFile(t, site, "tags/"+draftOnly+"/index.html")
	}

	k8s := page(t, site, "tags/k8s/index.html")
	assertContains(t, "k8s", k8s, "<title>#k8s · Test blog</title>")
	assertInOrder(t, "k8s", k8s, ">New<", ">Same day<", ">Old<")
	for _, other := range []string{">Untagged<", ">Scheduled<", ">Undated<"} {
		if strings.Contains(k8s, other) {
			t.Errorf("k8s lists %s:\n%s", other, k8s)
		}
	}
	talos := page(t, site, "tags/talos/index.html")
	assertContains(t, "talos", talos, ">New<")
	if strings.Contains(talos, ">Undated<") {
		t.Errorf("talos lists a Draft:\n%s", talos)
	}
}

func TestIncludingDraftsCountsThemInTags(t *testing.T) {
	repo := contentRepo(map[string]string{
		"posts/published.md": "---\ntitle: Published\ndate: 2026-03-01\ntags: [k8s]\n---\n",
		"posts/undated.md":   "---\ntitle: Undated\ntags: [k8s, wip]\n---\n",
	})

	site := build(t, repo, now, service.BuildOptions{IncludeDrafts: true})

	assertNoProblems(t, site)
	assertInOrder(t, "tags", page(t, site, "tags/index.html"), "#k8s</a> (2)", "#wip</a> (1)")
	assertInOrder(t, "k8s", page(t, site, "tags/k8s/index.html"), "draft", ">Undated<", ">Published<")
}

func TestPostsLinkTheirTagsToTheTagPages(t *testing.T) {
	repo := contentRepo(map[string]string{
		"posts/x.md": "---\ntitle: X\ndate: 2026-03-01\ntags: [k8s, talos]\n---\n",
	})

	site := build(t, repo, now, service.BuildOptions{})

	assertNoProblems(t, site)
	for _, name := range []string{"index.html", "posts/x/index.html", "tags/k8s/index.html"} {
		assertInOrder(t, name, page(t, site, name),
			`<a href="/tags/k8s/">#k8s</a>`, `<a href="/tags/talos/">#talos</a>`)
	}
}

func TestTagURLsAreLowercaseWithOtherCharactersAsHyphens(t *testing.T) {
	tests := []struct {
		tag      string
		wantFile string
	}{
		{"k8s", "tags/k8s/index.html"},
		{"Home Lab", "tags/home-lab/index.html"},
		{"  padded\t", "tags/padded/index.html"},
		{"k8s/talos", "tags/k8s-talos/index.html"},
		{"this & that?", "tags/this-that/index.html"},
		{"C++", "tags/c++/index.html"},
		{"Node.js", "tags/node.js/index.html"},
		{"snake_case", "tags/snake_case/index.html"},
		{"Café", "tags/café/index.html"},
	}
	for _, tt := range tests {
		t.Run(tt.tag, func(t *testing.T) {
			repo := contentRepo(map[string]string{
				"posts/x.md": "---\ntitle: X\ndate: 2026-03-01\ntags: [\"" + tt.tag + "\"]\n---\n",
			})

			site := build(t, repo, now, service.BuildOptions{})

			assertNoProblems(t, site)
			assertContains(t, tt.wantFile, page(t, site, tt.wantFile), ">X<")
		})
	}
}

func TestTagsWithTheSameURLAreOneTagNamedAsOnTheNewestPost(t *testing.T) {
	repo := contentRepo(map[string]string{
		"posts/old.md": "---\ntitle: Old\ndate: 2025-01-01\ntags: [homelab]\n---\n",
		"posts/new.md": "---\ntitle: New\ndate: 2026-03-01\ntags: [HomeLab, homelab]\n---\n",
	})

	site := build(t, repo, now, service.BuildOptions{})

	assertNoProblems(t, site)
	tags := page(t, site, "tags/index.html")
	assertContains(t, "tags", tags, `<a href="/tags/homelab/">#HomeLab</a> (2)`)
	if strings.Count(tags, "/tags/homelab/") != 1 {
		t.Errorf("tags lists homelab more than once:\n%s", tags)
	}
	assertInOrder(t, "homelab", page(t, site, "tags/homelab/index.html"), ">New<", ">Old<")
}

func TestTagsWithNoLettersOrDigitsSkipThePost(t *testing.T) {
	for _, tag := range []string{"", "  ", "++", "..", "-/-"} {
		t.Run(tag, func(t *testing.T) {
			repo := contentRepo(map[string]string{
				"posts/bad.md":  "---\ntitle: Bad\ndate: 2026-03-01\ntags: [ok, \"" + tag + "\"]\n---\n",
				"posts/fine.md": "---\ntitle: Fine\ndate: 2026-03-01\ntags: [ok]\n---\n",
			})

			site := build(t, repo, now, service.BuildOptions{})

			if len(site.Problems) != 1 {
				t.Fatalf("got problems %v, want exactly one for posts/bad.md", site.Problems)
			}
			problem := site.Problems[0].String()
			assertContains(t, "problem", problem, "posts/bad.md")
			assertContains(t, "problem", problem, "tag")
			assertNoFile(t, site, "posts/bad/index.html")
			assertContains(t, "tags", page(t, site, "tags/index.html"), "#ok</a> (1)")
		})
	}
}
