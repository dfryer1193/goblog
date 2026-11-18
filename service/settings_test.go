package service_test

import (
	"strings"
	"testing"
	"time"

	"github.com/dfryer1193/goblog/service"
)

func TestSiteSettingsTitleAndNavigationAppearOnEveryPage(t *testing.T) {
	repo := contentRepo(map[string]string{
		"site.yaml": "title: Werewolf Notes\nauthor: D. Author\ndescription: Homelab & other things\n" +
			"url: https://blog.example.com\ntimezone: America/New_York\n" +
			"nav:\n  - name: posts\n    link: /\n  - name: tags\n    link: /tags/\n  - name: about\n    link: /about/\n",
		"posts/x.md": "---\ntitle: X\ndate: 2026-01-01\n---\n",
	})

	site := build(t, repo, now, service.BuildOptions{})

	assertNoProblems(t, site)
	for _, name := range []string{"index.html", "posts/x/index.html"} {
		text := page(t, site, name)
		assertContains(t, name, text, "Werewolf Notes")
		assertContains(t, name, text, `content="Homelab &amp; other things"`)
		assertContains(t, name, text, `content="D. Author"`)
		assertInOrder(
			t,
			name,
			text,
			`<a href="/">posts</a>`,
			`<a href="/tags/">tags</a>`,
			`<a href="/about/">about</a>`,
		)
	}
	assertContains(t, "post", page(t, site, "posts/x/index.html"), "<title>X · Werewolf Notes</title>")
}

func TestMissingSiteSettingsIsAProblemButTheSiteBuildsWithDefaults(t *testing.T) {
	repo := contentRepo(map[string]string{"posts/x.md": "---\ntitle: X\ndate: 2026-01-01\n---\n"})
	delete(repo, "site.yaml")

	site := build(t, repo, now, service.BuildOptions{})

	if len(site.Problems) != 1 || !strings.HasPrefix(site.Problems[0].String(), "site.yaml: missing") {
		t.Fatalf("got problems %v, want one saying site.yaml is missing", site.Problems)
	}
	index := page(t, site, "index.html")
	assertContains(t, "index", index, "<title>goblog</title>")
	assertInOrder(t, "index", index, `<a href="/">posts</a>`, `<a href="/tags/">tags</a>`, ">X<")
}

func TestInvalidSiteSettingsFailTheBuild(t *testing.T) {
	tests := []struct {
		name     string
		settings string
		wantErr  string
	}{
		{"invalid YAML", "title: [unclosed\n", "invalid YAML"},
		{"no title", "author: Someone\n", "title"},
		{"blank title", "title: \"  \"\n", "title"},
		{"misspelt setting", "title: T\ntimzone: UTC\n", "timzone"},
		{"unknown timezone", "title: T\ntimezone: Mars/Olympus_Mons\n", "Mars/Olympus_Mons"},
		{"relative base URL", "title: T\nurl: blog.example.com\n", "blog.example.com"},
		{"non-web base URL", "title: T\nurl: ftp://blog.example.com\n", "ftp://blog.example.com"},
		{"nav entry without a link", "title: T\nnav:\n  - name: about\n", "nav entry 1"},
		{"nav entry without a name", "title: T\nnav:\n  - name: posts\n    link: /\n  - link: /x/\n", "nav entry 2"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := contentRepo(map[string]string{
				"site.yaml":  tt.settings,
				"posts/x.md": "---\ntitle: X\ndate: 2026-01-01\n---\n",
			})

			site, err := service.Build(repo, now, service.BuildOptions{})

			if err == nil {
				t.Fatalf("Build succeeded with files %v, want an error", site.Paths())
			}
			assertContains(t, "error", err.Error(), "site.yaml")
			assertContains(t, "error", err.Error(), tt.wantErr)
		})
	}
}

func TestSiteSettingsTimezoneDecidesPublishDatesOverTheBuildOption(t *testing.T) {
	newYork, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatalf("loading timezone: %v", err)
	}
	// 2026-03-10 in UTC, but still 23:59 on 2026-03-09 in New York.
	justBeforeMidnightInNewYork := time.Date(2026, time.March, 10, 3, 59, 0, 0, time.UTC)

	tests := []struct {
		name          string
		settings      string
		option        *time.Location
		wantPublished bool
	}{
		{"settings timezone", "title: T\nurl: https://t.example\ntimezone: America/New_York\n", nil, false},
		{"settings timezone beats the option", "title: T\nurl: https://t.example\ntimezone: UTC\n", newYork, true},
		{"option when the settings have none", "title: T\nurl: https://t.example\n", newYork, false},
		{"UTC when neither sets one", "title: T\nurl: https://t.example\n", nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := contentRepo(map[string]string{
				"site.yaml":      tt.settings,
				"posts/tenth.md": "---\ntitle: Tenth\ndate: 2026-03-10\n---\n",
			})

			site := build(t, repo, justBeforeMidnightInNewYork, service.BuildOptions{Timezone: tt.option})

			assertNoProblems(t, site)
			_, published := site.Files["posts/tenth/index.html"]
			if published != tt.wantPublished {
				t.Errorf("posts/tenth published = %v, want %v", published, tt.wantPublished)
			}
		})
	}
}
