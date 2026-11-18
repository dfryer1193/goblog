package service_test

import (
	"testing"
	"time"

	"github.com/dfryer1193/goblog/service"
)

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
	// 15:00 UTC is 11:00 in site.yaml's America/New_York, on daylight
	// saving time since 2026-03-08; the host is the first label of the
	// site's URL.
	assertInOrder(t, "status bar", bar,
		`<details class="ham"><summary aria-label="files">☰</summary>`, "</details>",
		`[Werewolf Notes]`, sep, windowsTag, "</nav>",
		`<span class="right">`+sep+"2026.03.10"+sep+"11:00 EDT"+sep+"blog"+sep+"</span>")
}

func TestStatusBarTimeHasTheTimezoneAbbreviationForTheRenderTime(t *testing.T) {
	winter := time.Date(2026, time.January, 20, 23, 30, 0, 0, time.UTC)

	site := build(t, designRepo(), winter, service.BuildOptions{})

	assertContains(t, "status bar", page(t, site, "index.html"),
		`<span class="right">`+sep+"2026.01.20"+sep+"18:30 EST"+sep+"blog"+sep+"</span>")
}

func TestStatusBarHostIsGoblogWithoutSiteSettings(t *testing.T) {
	repo := contentRepo(nil)
	delete(repo, "site.yaml")
	site := build(t, repo, now, service.BuildOptions{})

	assertContains(t, "status bar", page(t, site, "index.html"),
		`<span class="right">`+sep+"2026.03.10"+sep+"15:00 UTC"+sep+"goblog"+sep+"</span>")
}
