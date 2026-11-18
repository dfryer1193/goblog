package service

import (
	"strings"

	"github.com/dfryer1193/goblog/models"
)

// feedPath is the site path of the Atom feed.
const feedPath = "/feed.xml"

// postsWindow is the name of the navigation entry (the status bar window)
// that the index and the Post pages belong to. It's also the index's pane
// title.
const postsWindow = "posts"

// Tree drawing, as `tree` draws it.
const (
	treeEntry = "├── "
	treeLast  = "└── "
	treeDepth = "│   "
)

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
}

// makeTree lays out the tree pane for the listed posts, their tags and the
// Pages. Lines are unselected; treeFor selects the page being shown.
func makeTree(posts []*models.Post, tags []*models.Tag, pages []*models.Page) []treeLine {
	var lines []treeLine
	lines = append(lines, treeLine{Prefix: treeEntry, Name: postsDir + "/", URL: "/"})
	for i, p := range posts {
		lines = append(
			lines,
			treeLine{Prefix: treeDepth + branch(i, len(posts)), Name: p.Slug + markdownExt, URL: p.URL()},
		)
	}
	lines = append(lines, treeLine{Prefix: treeEntry, Name: tagsSection + "/", URL: "/" + tagsSection + "/"})
	for i, tag := range tags {
		lines = append(
			lines,
			treeLine{Prefix: treeDepth + branch(i, len(tags)), Name: tag.Name, URL: tag.URL(), Class: "tag"},
		)
	}
	for _, p := range pages {
		lines = append(lines, treeLine{Prefix: treeEntry, Name: p.Name + markdownExt, URL: p.URL()})
	}
	lines = append(lines, treeLine{Prefix: treeLast, Name: strings.TrimPrefix(feedPath, "/"), URL: feedPath})
	return lines
}

// branch is the tree drawing for entry i of n in a folder.
func branch(i, n int) string {
	if i == n-1 {
		return treeLast
	}
	return treeEntry
}

// treeFor returns tree with the entry for the page at sitePath selected.
func treeFor(tree []treeLine, sitePath string) []treeLine {
	lines := make([]treeLine, len(tree))
	for i, line := range tree {
		if line.URL == sitePath {
			line.Current = true
			line.Class = strings.TrimSpace(line.Class + " sel")
		}
		lines[i] = line
	}
	return lines
}

// sitePath is the site path that serves the output file name: "/" for
// index.html, "/x/" for x/index.html, otherwise "/" + name.
func sitePath(name string) string {
	if name == "index.html" {
		return "/"
	}
	if dir, ok := strings.CutSuffix(name, "/index.html"); ok {
		return "/" + dir + "/"
	}
	return "/" + name
}
