package models

import (
	"strings"
	"unicode"
)

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

// TagSlug is the URL path segment for the tag called name: name in lower
// case, keeping letters, digits, '+', '.' and '_', with every other run of
// characters (spaces, '/', '&' and so on) turned into one '-'. Leading and
// trailing '-' and '.' are dropped. Tags with the same slug are the same tag.
// It's "" if name has no letters or digits, and such a tag is invalid.
func TagSlug(name string) string {
	var b strings.Builder
	hyphen := false
	for _, r := range strings.ToLower(name) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || strings.ContainsRune("+._", r) {
			if hyphen && b.Len() > 0 {
				b.WriteByte('-')
			}
			hyphen = false
			b.WriteRune(r)
			continue
		}
		hyphen = true
	}
	slug := strings.Trim(b.String(), "-.")
	if !strings.ContainsFunc(slug, func(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }) {
		return ""
	}
	return slug
}
