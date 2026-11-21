package service

import (
	"math"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/dfryer1193/goblog/models"
)

// feedPath is the site path of the Atom feed.
const feedPath = "/" + models.FeedFile

// postsWindow is the name of the navigation entry (the status bar window)
// that the index and the Post pages belong to. It's also the index's pane
// title.
const postsWindow = "posts"

// Tree drawing, as `tree` draws it.
const (
	treeEntry = "├── "
	treeLast  = "└── "
	treeDepth = "│   "
	treeBlank = "    "
)

// treeFolderShown is how many entries a tree folder shows before an
// ellipsis that opens to the rest.
const treeFolderShown = 20

// treeLine is one line of the tree pane, which lists the site like `tree`
// lists a directory: Posts, tags, Pages and the feed.
type treeLine struct {
	// Prefix is the line's tree drawing, e.g. "│   ├── ".
	Prefix string
	// Name is the entry's name, e.g. "picluster.md".
	Name string
	// URL is where the entry links to.
	URL string
	// Class is the link's CSS class: "tag" for a tag, plus "sel" for the
	// page being shown.
	Class string
	// Current reports whether the entry is the page being shown.
	Current bool

	// Fold reports whether the line heads a folder with entries under it,
	// which then collapse into it: the line is a <details>' summary.
	Fold bool
	// More, if not "", is the tree drawing of an ellipsis line before this
	// one, standing for this entry and the rest of its folder: a <details>
	// whose summary is the ellipsis, so it opens to them with no JavaScript.
	More string
	// MoreOpen reports whether that ellipsis starts open, because it hides
	// the page being shown.
	MoreOpen bool
	// EndMore and EndFold report whether the line is the last under an
	// ellipsis and under a folder, closing their <details>.
	EndMore, EndFold bool
}

// makeTree lays out the tree pane for the listed posts, their tags, the
// Pages and the feed at feed, if the site has one (feed isn't ""). Lines are
// unselected; treeFor selects the page being shown.
func makeTree(posts []*models.Post, tags []*models.Tag, pages []*models.Page, feed string) []treeLine {
	postsFolder := treeFolder{entry: treeLine{Name: postsDir + "/", URL: "/"}}
	for _, p := range posts {
		postsFolder.children = append(postsFolder.children, treeLine{Name: p.Slug + markdownExt, URL: p.URL()})
	}
	tagsFolder := treeFolder{entry: treeLine{Name: models.TagsSection + "/", URL: tagsURL}}
	for _, tag := range tags {
		tagsFolder.children = append(tagsFolder.children, treeLine{Name: tag.Name, URL: tag.URL(), Class: "tag"})
	}
	top := []treeFolder{postsFolder, tagsFolder}
	for _, p := range pages {
		top = append(top, treeFolder{entry: treeLine{Name: p.Name + markdownExt, URL: p.URL()}})
	}
	if feed != "" {
		top = append(top, treeFolder{entry: treeLine{Name: path.Base(feed), URL: feed}})
	}

	var lines []treeLine
	for i, folder := range top {
		last := i == len(top)-1
		folder.entry.Prefix = branch(i, len(top))
		lines = append(lines, folder.entry)
		indent := treeDepth
		if last {
			indent = treeBlank
		}
		n := len(folder.children)
		for j, child := range folder.children {
			child.Prefix = indent + branch(j, n)
			if j == treeFolderShown && n > treeFolderShown {
				child.More = indent + treeLast
			}
			child.EndMore = j == n-1 && n > treeFolderShown
			child.EndFold = j == n-1
			lines = append(lines, child)
		}
		if n > 0 {
			lines[len(lines)-n-1].Fold = true
		}
	}
	return lines
}

// treeFolder is a top-level entry of the tree pane and the entries under it.
type treeFolder struct {
	entry    treeLine
	children []treeLine
}

// branch is the tree drawing for entry i of n in a folder.
func branch(i, n int) string {
	if i == n-1 {
		return treeLast
	}
	return treeEntry
}

// treeFor returns tree with the entry for the page at sitePath selected,
// and the ellipsis hiding it, if any, open.
func treeFor(tree []treeLine, sitePath string) []treeLine {
	lines := make([]treeLine, len(tree))
	more := -1 // the line that starts the ellipsis the loop is under
	for i, line := range tree {
		if line.More != "" {
			more = i
		}
		if line.URL == sitePath {
			line.Current = true
			line.Class = strings.TrimSpace(line.Class + " sel")
			if more >= 0 {
				lines[more].MoreOpen = true
				if more == i {
					line.MoreOpen = true
				}
			}
		}
		if line.EndMore {
			more = -1
		}
		lines[i] = line
	}
	return lines
}

// indexFileFor is the output file that serves the site path sitePath, which
// ends in "/": "index.html" for "/", "x/index.html" for "/x/". It's the
// inverse of sitePath for such paths.
func indexFileFor(sitePath string) string {
	return strings.TrimPrefix(sitePath, "/") + models.IndexFile
}

// sitePath is the site path that serves the output file name: "/" for
// index.html, "/x/" for x/index.html, otherwise "/" + name.
func sitePath(name string) string {
	if name == models.IndexFile {
		return "/"
	}
	if dir, ok := strings.CutSuffix(name, "/"+models.IndexFile); ok {
		return "/" + dir + "/"
	}
	return "/" + name
}

// defaultHost is the status bar's host when site.yaml sets neither host nor
// url.
const defaultHost = "goblog"

// buildInfo is what the status bar and the info line say about the build.
type buildInfo struct {
	// Date and Time are the render time in the site's timezone, as
	// YYYY.MM.DD and as HH:MM with the zone's abbreviation at that time,
	// e.g. "11:00 EDT".
	Date, Time string
	// Host is site.yaml's host if set, otherwise the first DNS label of the
	// site's URL, e.g. "blog" for https://blog.example.com, or "goblog" if
	// it has none.
	Host string
	// Commit is the short hash of the Content Repository commit rendered,
	// or "" if it's unknown.
	Commit string
}

// shortCommitLen is how many hex digits of the commit the info line shows.
const shortCommitLen = 7

// newBuildInfo describes a build rendered at now, in the site's timezone, of
// the given commit, for a site with the given settings.
func newBuildInfo(now time.Time, tz *time.Location, settings models.SiteSettings, commit string) buildInfo {
	host := settings.Host
	if host == "" {
		host = defaultHost
		if u, err := url.Parse(settings.URL); err == nil && u.Hostname() != "" {
			host, _, _ = strings.Cut(u.Hostname(), ".")
		}
	}
	local := now.In(tz)
	return buildInfo{
		Date:   local.Format("2006.01.02"),
		Time:   local.Format("15:04 MST"),
		Host:   host,
		Commit: commit[:min(len(commit), shortCommitLen)],
	}
}

// window is one status bar window: a navigation entry.
type window struct {
	// Index is the window's number, from 0.
	Index int
	// Name and Link are the navigation entry's.
	Name, Link string
	// Current reports whether the page being shown belongs to the window.
	Current bool
	// Title is the window's #T, the page's name within the window, when
	// it's current.
	Title string
}

// windowsFor lays out the status bar windows for a page. The first window
// the page belongs to is current; see belongsTo.
func windowsFor(nav []models.NavLink, data *pageData) []window {
	windows := make([]window, len(nav))
	found := false
	for i, link := range nav {
		windows[i] = window{Index: i, Name: link.Name, Link: link.Link}
		if !found && belongsTo(link, data) {
			found = true
			windows[i].Current = true
			windows[i].Title = data.WindowTitle
		}
	}
	return windows
}

// belongsTo reports whether the page belongs to the navigation entry link:
// Post pages belong to the posts window; another page belongs to the entry
// that links to it, or to a section (a link ending in "/") that holds it.
// The 404 page belongs to no window.
func belongsTo(link models.NavLink, data *pageData) bool {
	switch {
	case data.NotFound:
		return false
	case data.Post != nil:
		return link.Name == postsWindow
	case link.Link == data.Path:
		return true
	default:
		return link.Link != "/" && strings.HasSuffix(link.Link, "/") && strings.HasPrefix(data.Path, link.Link)
	}
}

// readingSpeed is how many words a minute the info line's reading time
// assumes.
const readingSpeed = 230

// readingMinutes is how long n words take to read at readingSpeed, rounded
// to the nearest minute but at least one.
func readingMinutes(words int) int {
	return max(1, int(math.Round(float64(words)/readingSpeed)))
}

// count formats n of something, e.g. count(1, "post", "posts") is "1 post"
// and count(1234, "word", "words") is "1,234 words".
func count(n int, one, many string) string {
	digits := strconv.Itoa(n)
	var b strings.Builder
	for i, d := range digits {
		if i > 0 && (len(digits)-i)%3 == 0 && digits[i-1] != '-' {
			b.WriteByte(',')
		}
		b.WriteRune(d)
	}
	if n == 1 {
		return b.String() + " " + one
	}
	return b.String() + " " + many
}
