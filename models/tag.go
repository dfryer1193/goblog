package models

import "strings"

// Tag is one tag and the listed Posts that have it, in index order.
type Tag struct {
	// Name is the tag as the author wrote it.
	Name string
	// Posts are the Posts with the tag, newest first.
	Posts []*Post
}

// URL is the tag's site path, e.g. /tags/k8s/.
func (t *Tag) URL() string {
	return TagURL(t.Name)
}

// TagURL is the site path of the tag called name, e.g. /tags/k8s/.
func TagURL(name string) string {
	return "/tags/" + TagSlug(name) + "/"
}

// TagSlug is the URL path segment for the tag called name.
func TagSlug(name string) string {
	return strings.ToLower(name)
}
