package stats

import (
	"net/http"
	"strings"

	"github.com/dfryer1193/goblog/models"
)

// The kinds of page a request can be for.
const (
	kindPost  = "post"
	kindPage  = "page"
	kindIndex = "index"
	kindTag   = "tag"
	kindOther = "other"
)

// kindOf is the kind of page at urlPath, answered with status.
func kindOf(urlPath string, _ int) string {
	if urlPath == "/" {
		return kindIndex
	}
	section, rest, _ := strings.Cut(strings.TrimPrefix(urlPath, "/"), "/")
	switch {
	case section == models.PostsSection && isFolder(rest):
		return kindPost
	case section == models.TagsSection && (rest == "" || isFolder(rest)):
		return kindTag
	case section != "" && rest == "" && strings.HasSuffix(urlPath, "/"):
		return kindPage
	default:
		return kindOther
	}
}

// isFolder reports whether rest is one folder's URL, such as "hello/".
func isFolder(rest string) bool {
	name, ok := strings.CutSuffix(rest, "/")
	return ok && name != "" && !strings.Contains(name, "/")
}

// isHTMLPage reports whether kind is a kind of HTML page.
func isHTMLPage(kind string) bool {
	return kind == kindPost || kind == kindPage || kind == kindIndex || kind == kindTag
}

// isPageView reports whether r, for a page of kind answered with status, is
// a Page View.
func isPageView(r *http.Request, kind string, status int) bool {
	return r.Method == http.MethodGet && isHTMLPage(kind) && status == http.StatusOK
}
