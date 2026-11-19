package api

import (
	"mime"
	"path"
	"strings"

	"github.com/dfryer1193/goblog/models"
)

// feedContentType is the Atom feed's media type (RFC 4287).
const feedContentType = "application/atom+xml; charset=utf-8"

// siteContentTypes are the media types of the text files the site build
// writes, fixed so they don't depend on the host's MIME table.
var siteContentTypes = map[string]string{ //nolint:gochecknoglobals // fixed set
	".html": "text/html; charset=utf-8",
	".css":  "text/css; charset=utf-8",
	".xml":  "application/xml; charset=utf-8",
	".txt":  "text/plain; charset=utf-8",
}

// contentType is the Content-Type for serving the site file name, which may
// be gzipped: the feed is Atom, the site's own text files have fixed types,
// and anything else goes by its extension.
func contentType(name string) string {
	if name == models.FeedFile {
		return feedContentType
	}
	ext := strings.ToLower(path.Ext(name))
	if t, ok := siteContentTypes[ext]; ok {
		return t
	}
	if t := mime.TypeByExtension(ext); t != "" {
		return t
	}
	return "application/octet-stream"
}
