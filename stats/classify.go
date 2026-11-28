package stats

import (
	"net/http"
	"strings"

	"github.com/dfryer1193/goblog/models"
)

// The kinds of page a request can be for.
const (
	kindPost    = "post"
	kindPage    = "page"
	kindIndex   = "index"
	kindTag     = "tag"
	kindImage   = "image"
	kindAsset   = "asset"
	kindFeed    = "feed"
	kindSitemap = "sitemap"
	kindMissing = "missing"
	kindOther   = "other"
)

// kindOf is the kind of page at urlPath, answered with status.
func kindOf(urlPath string, status int) string {
	if status == http.StatusNotFound {
		return kindMissing
	}
	switch urlPath {
	case "/":
		return kindIndex
	case "/" + models.FeedFile:
		return kindFeed
	case "/" + models.SitemapFile:
		return kindSitemap
	}
	section, rest, _ := strings.Cut(strings.TrimPrefix(urlPath, "/"), "/")
	switch {
	case section == models.ImagesSection && rest != "":
		return kindImage
	case (section == models.AssetsSection || section == models.FontsSection || section == models.LicensesSection) &&
		rest != "":
		return kindAsset
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

// isPageView reports whether req is a Page View: a GET that isn't a
// prefetch, for an HTML page, answered 200 or 304.
func (req request) isPageView() bool {
	return req.method == http.MethodGet &&
		!req.isPrefetch() &&
		isHTMLPage(req.kind) &&
		(req.status == http.StatusOK || req.status == http.StatusNotModified)
}

// isVisitorNotFound reports whether req, a Visitor's request, is a Visitor
// 404: a GET that isn't a prefetch, answered with the 404 page. Like a Page
// View, a HEAD isn't one.
func (req request) isVisitorNotFound() bool {
	return req.method == http.MethodGet && !req.isPrefetch() && req.kind == kindMissing
}
